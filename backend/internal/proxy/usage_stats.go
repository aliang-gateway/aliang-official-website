package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
)

// AdminUsageGroup is the group object embedded in each admin usage record.
type AdminUsageGroup struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	RateMultiplier float64 `json:"rate_multiplier"`
}

// AdminUsageRecord is one billing row from GET /api/v1/admin/usage. Only the
// fields the cache-rate aggregation needs are declared; the upstream returns
// considerably more (costs, subscription, account, ...).
type AdminUsageRecord struct {
	ID                  int64            `json:"id"`
	GroupID             int64            `json:"group_id"`
	InputTokens         int64            `json:"input_tokens"`
	OutputTokens        int64            `json:"output_tokens"`
	CacheReadTokens     int64            `json:"cache_read_tokens"`
	CacheCreationTokens int64            `json:"cache_creation_tokens"`
	Group               *AdminUsageGroup `json:"group,omitempty"`
}

// AdminUsagePage is one page of the upstream admin usage listing. The
// upstream clamps per_page to 20, so callers must paginate.
type AdminUsagePage struct {
	Records    []AdminUsageRecord
	Pagination Pagination
}

// GetAdminUsagePage fetches one page of admin usage records. groupID 0 means
// "all groups"; empty startDate/endDate are omitted from the query.
func (c *Client) GetAdminUsagePage(ctx context.Context, page, perPage int, groupID int64, startDate, endDate string) (*AdminUsagePage, error) {
	path := "/api/v1/admin/usage?page=" + url.QueryEscape(strconv.Itoa(page)) + "&per_page=" + url.QueryEscape(strconv.Itoa(perPage))
	if groupID > 0 {
		path += "&group_id=" + url.QueryEscape(strconv.FormatInt(groupID, 10))
	}
	if startDate != "" {
		path += "&start_date=" + url.QueryEscape(startDate)
	}
	if endDate != "" {
		path += "&end_date=" + url.QueryEscape(endDate)
	}

	// 上游把分页字段放在顶层（{data:[...], total, page, page_size, pages}），
	// 与 ListResponseEnvelope 的嵌套 pagination 形态不同，这里单独宽容解析。
	var raw struct {
		Data       json.RawMessage `json:"data"`
		Items      json.RawMessage `json:"items"`
		Total      int             `json:"total"`
		Page       int             `json:"page"`
		PageSize   int             `json:"page_size"`
		Pages      int             `json:"pages"`
		Pagination Pagination      `json:"pagination"`
	}
	if err := c.DoAdminJSON(ctx, AdminRequest{Method: http.MethodGet, Path: path}, &raw); err != nil {
		return nil, err
	}

	listRaw := raw.Data
	if len(bytes.TrimSpace(raw.Items)) > 0 {
		listRaw = raw.Items
	}
	var records []AdminUsageRecord
	if len(bytes.TrimSpace(listRaw)) > 0 {
		if err := json.Unmarshal(listRaw, &records); err != nil {
			return nil, err
		}
	}

	pagination := raw.Pagination
	if pagination.Total == 0 && pagination.Page == 0 && pagination.Pages == 0 {
		pagination = Pagination{Total: raw.Total, Page: raw.Page, PageSize: raw.PageSize, Pages: raw.Pages}
	}
	return &AdminUsagePage{Records: records, Pagination: pagination}, nil
}

// GroupCacheRateStats is the per-group cache hit-rate breakdown.
type GroupCacheRateStats struct {
	GroupID             int64   `json:"group_id"`
	GroupName           string  `json:"group_name"`
	RateMultiplier      float64 `json:"rate_multiplier"`
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheHitRate        float64 `json:"cache_hit_rate"`
}

// CacheRateStats is the site-wide (or group-filtered) cache hit-rate summary
// behind GET /admin/cache-stats. Hit rate口径：cache_read /
// (input + cache_read + cache_creation)。
type CacheRateStats struct {
	Days                int                   `json:"days"`
	StartDate           string                `json:"start_date"`
	EndDate             string                `json:"end_date"`
	Requests            int64                 `json:"requests"`
	InputTokens         int64                 `json:"input_tokens"`
	CacheReadTokens     int64                 `json:"cache_read_tokens"`
	CacheCreationTokens int64                 `json:"cache_creation_tokens"`
	CacheHitRate        float64               `json:"cache_hit_rate"`
	Groups              []GroupCacheRateStats `json:"groups"`
}

// AggregateUsageCacheStats reduces raw usage records into overall plus
// per-group cache-rate stats. Groups are ordered by request count descending.
func AggregateUsageCacheStats(records []AdminUsageRecord) CacheRateStats {
	stats := CacheRateStats{}
	type groupAcc struct {
		name                            string
		multiplier                      float64
		requests, input, read, creation int64
	}
	groups := map[int64]*groupAcc{}

	for _, record := range records {
		stats.Requests++
		stats.InputTokens += record.InputTokens
		stats.CacheReadTokens += record.CacheReadTokens
		stats.CacheCreationTokens += record.CacheCreationTokens

		acc := groups[record.GroupID]
		if acc == nil {
			acc = &groupAcc{}
			groups[record.GroupID] = acc
		}
		acc.requests++
		acc.input += record.InputTokens
		acc.read += record.CacheReadTokens
		acc.creation += record.CacheCreationTokens
		if record.Group != nil {
			acc.name = record.Group.Name
			acc.multiplier = record.Group.RateMultiplier
		}
	}

	total := stats.InputTokens + stats.CacheReadTokens + stats.CacheCreationTokens
	if total > 0 {
		stats.CacheHitRate = float64(stats.CacheReadTokens) / float64(total)
	}

	for groupID, acc := range groups {
		denominator := acc.input + acc.read + acc.creation
		hitRate := 0.0
		if denominator > 0 {
			hitRate = float64(acc.read) / float64(denominator)
		}
		stats.Groups = append(stats.Groups, GroupCacheRateStats{
			GroupID:             groupID,
			GroupName:           acc.name,
			RateMultiplier:      acc.multiplier,
			Requests:            acc.requests,
			InputTokens:         acc.input,
			CacheReadTokens:     acc.read,
			CacheCreationTokens: acc.creation,
			CacheHitRate:        hitRate,
		})
	}
	sort.Slice(stats.Groups, func(i, j int) bool {
		if stats.Groups[i].Requests != stats.Groups[j].Requests {
			return stats.Groups[i].Requests > stats.Groups[j].Requests
		}
		return stats.Groups[i].GroupID < stats.Groups[j].GroupID
	})
	return stats
}
