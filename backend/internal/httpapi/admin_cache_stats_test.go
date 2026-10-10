package httpapi

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"ai-api-portal/backend/internal/proxy"
)

// 上游 mock：page=1 返回 2 条（pages=2），page=2 返回 1 条（pages=2），
// 用于验证 handler 跨页聚合。
func newCacheStatsTestRoutes(t *testing.T) (*http.ServeMux, *sql.DB, *atomic.Int64, *atomic.Value) {
	t.Helper()

	var upstreamCalls atomic.Int64
	var lastQuery atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		lastQuery.Store(r.URL.RawQuery)
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		if page == "2" {
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":3,"group_id":28,"input_tokens":200,"cache_read_tokens":800,"cache_creation_tokens":0,` +
				`"group":{"id":28,"name":"openai-balance","rate_multiplier":1.5}}],"total":3,"page":2,"page_size":2,"pages":2}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"data":[` +
			`{"id":1,"group_id":22,"input_tokens":1000,"cache_read_tokens":9000,"cache_creation_tokens":0,` +
			`"group":{"id":22,"name":"anthropic-duration","rate_multiplier":1}},` +
			`{"id":2,"group_id":22,"input_tokens":500,"cache_read_tokens":0,"cache_creation_tokens":1500,` +
			`"group":{"id":22,"name":"anthropic-duration","rate_multiplier":1}}],` +
			`"total":3,"page":1,"page_size":2,"pages":2}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	database := setupTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", SQLDialect: "sqlite", ProxyClient: proxyClient})
	return mux, database, &upstreamCalls, &lastQuery
}

func TestAdminCacheStatsAggregatesAcrossPages(t *testing.T) {
	t.Parallel()

	mux, database, upstreamCalls, lastQuery := newCacheStatsTestRoutes(t)
	adminToken := seedUserWithSession(t, database, "cache-admin@example.com", "admin")

	req := httptest.NewRequest(http.MethodGet, "/admin/cache-stats?days=3", nil)
	setBearerAuth(req, adminToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"cache_hit_rate"`, `"groups"`, `"anthropic-duration"`, `"openai-balance"`, `"days":3`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %s: %s", want, body)
		}
	}
	// 上游被翻页调用了 2 次（pages=2），query 带 start_date/end_date
	if calls := upstreamCalls.Load(); calls < 2 {
		t.Fatalf("expected >=2 upstream page calls, got %d", calls)
	}
	if q, _ := lastQuery.Load().(string); !strings.Contains(q, "start_date=") || !strings.Contains(q, "end_date=") {
		t.Fatalf("upstream query missing date range: %q", q)
	}
}

func TestAdminCacheStatsRequiresAdmin(t *testing.T) {
	t.Parallel()

	mux, database, _, _ := newCacheStatsTestRoutes(t)
	userToken := seedUserWithSession(t, database, "cache-pleb@example.com", "user")

	req := httptest.NewRequest(http.MethodGet, "/admin/cache-stats", nil)
	setBearerAuth(req, userToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for plain user, got %d", rec.Code)
	}
}

func TestAdminCacheStatsClampsDays(t *testing.T) {
	t.Parallel()

	mux, database, _, lastQuery := newCacheStatsTestRoutes(t)
	adminToken := seedUserWithSession(t, database, "clamp-admin@example.com", "admin")

	for _, tc := range []struct {
		query string
		want  string
	}{
		{"/admin/cache-stats", `"days":3`},
		{"/admin/cache-stats?days=0", `"days":3`},
		{"/admin/cache-stats?days=999", `"days":7`},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.query, nil)
		setBearerAuth(req, adminToken)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d: %s", tc.query, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("%s: body missing %s: %s", tc.query, tc.want, rec.Body.String())
		}
	}
	if _, ok := lastQuery.Load().(string); !ok {
		t.Fatal("expected upstream to have been called")
	}
}
