package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-api-portal/backend/internal/proxy"
)

// handleAdminCacheStats serves GET /admin/cache-stats?days=N&group_id=ID —
// the site-wide (or per-group) prompt-cache hit rate. The upstream has no
// server-side aggregation endpoint and clamps per_page to 20, so this
// paginates the admin usage listing with the portal's admin key and
// aggregates locally; days is capped at 7 to bound the request count.
func (r *routes) handleAdminCacheStats(w http.ResponseWriter, req *http.Request) {
	if r.proxyClient == nil {
		writeError(w, http.StatusInternalServerError, "proxy is not configured")
		return
	}

	days := 3
	if raw := strings.TrimSpace(req.URL.Query().Get("days")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			days = parsed
		}
	}
	if days < 1 {
		days = 3
	}
	if days > 7 {
		days = 7
	}

	groupID := int64(0)
	if raw := strings.TrimSpace(req.URL.Query().Get("group_id")); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			groupID = parsed
		}
	}

	end := time.Now().UTC()
	startDate := end.AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	endDate := end.Format("2006-01-02")

	records := make([]proxy.AdminUsageRecord, 0, 64)
	const maxPages = 50 // 与 days=7 的上游翻页量相比留足余量，防御异常分页
	for page := 1; page <= maxPages; page++ {
		result, err := r.proxyClient.GetAdminUsagePage(req.Context(), page, 20, groupID, startDate, endDate)
		if err != nil {
			slog.Error("admin cache stats upstream fetch failed", "page", page, "error", err)
			writeError(w, http.StatusBadGateway, "failed to fetch usage from upstream")
			return
		}
		records = append(records, result.Records...)
		if len(result.Records) == 0 || result.Pagination.Page >= result.Pagination.Pages {
			break
		}
	}

	stats := proxy.AggregateUsageCacheStats(records)
	stats.Days = days
	stats.StartDate = startDate
	stats.EndDate = endDate
	writeJSON(w, http.StatusOK, stats)
}
