package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetAdminUsagePage_ParsesRecordsAndPagination(t *testing.T) {
	t.Parallel()

	var seenQuery string
	var seenKeyHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenQuery = r.URL.RawQuery
		seenKeyHeader = r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":[` +
			`{"id":421299,"group_id":22,"input_tokens":1525,"output_tokens":900,"cache_read_tokens":2886080,"cache_creation_tokens":0,` +
			`"group":{"id":22,"name":"anthropic-duration","rate_multiplier":1}},` +
			`{"id":421300,"group_id":28,"input_tokens":100,"output_tokens":50,"cache_read_tokens":0,"cache_creation_tokens":20,` +
			`"group":{"id":28,"name":"openai-balance","rate_multiplier":1.5}}],` +
			`"total":22,"page":2,"page_size":20,"pages":2}`))
	}))
	defer upstream.Close()

	client, err := NewClientWithOptions(upstream.URL, &http.Client{Timeout: RequestTimeout}, ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	page, err := client.GetAdminUsagePage(context.Background(), 2, 20, 0, "2026-10-09", "2026-10-10")
	if err != nil {
		t.Fatalf("GetAdminUsagePage() error = %v", err)
	}
	if seenKeyHeader != "admin-key-123" {
		t.Fatalf("admin calls must use x-api-key header, got %q", seenKeyHeader)
	}
	for _, want := range []string{"page=2", "per_page=20", "start_date=2026-10-09", "end_date=2026-10-10"} {
		if !strings.Contains(seenQuery, want) {
			t.Fatalf("query %q missing %q", seenQuery, want)
		}
	}
	if len(page.Records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(page.Records))
	}
	if page.Records[0].CacheReadTokens != 2886080 || page.Records[0].Group == nil || page.Records[0].Group.Name != "anthropic-duration" {
		t.Fatalf("record 0 parse mismatch: %#v", page.Records[0])
	}
	if page.Pagination.Total != 22 || page.Pagination.Page != 2 || page.Pagination.Pages != 2 {
		t.Fatalf("pagination mismatch: %#v", page.Pagination)
	}
}

func TestGetAdminUsagePage_GroupFilterOmittedWhenZero(t *testing.T) {
	t.Parallel()

	var seenQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[],"total":0,"page":1,"page_size":20,"pages":0}`))
	}))
	defer upstream.Close()

	client, _ := NewClientWithOptions(upstream.URL, &http.Client{Timeout: RequestTimeout}, ClientOptions{AdminAPIKey: "k"})
	if _, err := client.GetAdminUsagePage(context.Background(), 1, 20, 0, "", ""); err != nil {
		t.Fatalf("GetAdminUsagePage() error = %v", err)
	}
	if strings.Contains(seenQuery, "group_id") {
		t.Fatalf("group_id=0 must be omitted, got %q", seenQuery)
	}
}

func TestAggregateUsageCacheStats(t *testing.T) {
	t.Parallel()

	records := []AdminUsageRecord{
		{GroupID: 22, InputTokens: 1000, CacheReadTokens: 9000, CacheCreationTokens: 0, Group: &AdminUsageGroup{ID: 22, Name: "anthropic-duration", RateMultiplier: 1}},
		{GroupID: 22, InputTokens: 500, CacheReadTokens: 0, CacheCreationTokens: 1500, Group: &AdminUsageGroup{ID: 22, Name: "anthropic-duration", RateMultiplier: 1}},
		{GroupID: 28, InputTokens: 200, CacheReadTokens: 800, CacheCreationTokens: 0, Group: &AdminUsageGroup{ID: 28, Name: "openai-balance", RateMultiplier: 1.5}},
		{GroupID: 28, InputTokens: 0, CacheReadTokens: 0, CacheCreationTokens: 0}, // 无 group 信息的记录归入未知组
	}

	stats := AggregateUsageCacheStats(records)

	if stats.Requests != 4 {
		t.Fatalf("requests = %d, want 4", stats.Requests)
	}
	wantInput := int64(1000 + 500 + 200)
	if stats.InputTokens != wantInput {
		t.Fatalf("input tokens = %d, want %d", stats.InputTokens, wantInput)
	}
	if stats.CacheReadTokens != 9800 || stats.CacheCreationTokens != 1500 {
		t.Fatalf("cache token mismatch: read=%d creation=%d", stats.CacheReadTokens, stats.CacheCreationTokens)
	}
	if stats.CacheHitRate <= 0 || stats.CacheHitRate >= 1 {
		t.Fatalf("hit rate %v should be in (0,1)", stats.CacheHitRate)
	}

	if len(stats.Groups) == 0 {
		t.Fatal("expected per-group breakdown")
	}
	// 组按请求数降序：group 22 (2 条) 在前
	if stats.Groups[0].GroupID != 22 || stats.Groups[0].Requests != 2 {
		t.Fatalf("groups[0] = %#v, want group 22 with 2 requests", stats.Groups[0])
	}
	if stats.Groups[0].CacheHitRate <= 0 || stats.Groups[0].CacheHitRate >= 1 {
		t.Fatalf("group hit rate %v out of range", stats.Groups[0].CacheHitRate)
	}
}

func TestAggregateUsageCacheStats_Empty(t *testing.T) {
	t.Parallel()

	stats := AggregateUsageCacheStats(nil)
	if stats.Requests != 0 || stats.CacheHitRate != 0 || len(stats.Groups) != 0 {
		t.Fatalf("empty aggregation mismatch: %#v", stats)
	}
}
