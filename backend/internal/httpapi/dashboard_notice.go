package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"ai-api-portal/backend/internal/auth"
	"ai-api-portal/backend/internal/db"
)

// Dashboard 公告（portal 自有，非上游透传）：单条当前公告，admin 在后台编辑，
// 登录用户进入控制台时弹窗，未点击确认则每次进入都弹。
//   - 内容存 als_portal_settings（KV，admin 可改免重启，见 0031 先例）；
//   - 确认按账号记录在 als_dashboard_notice_acks（0032），键为公告内容哈希 id
//     ——admin 改文案即换 id，所有用户（含已确认过的）重新弹；仅切换 enabled
//     不改 id，不触发重弹。

const portalSettingDashboardNotice = "dashboard_notice"

type dashboardNotice struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Enabled   bool   `json:"enabled"`
	UpdatedAt string `json:"updated_at"`
}

// noticeIDFor 由内容哈希生成公告 id：内容不变则 id 稳定，改文案即换 id。
func noticeIDFor(title, body string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(title) + "\n" + strings.TrimSpace(body)))
	return hex.EncodeToString(sum[:])[:16]
}

// loadDashboardNotice 读取存储的公告（含未启用的，供 admin 端回显）。缺记录或
// 记录损坏返回 nil——损坏时显式当作空公告，不让坏数据静默滚成 stale 内容。
func (r *routes) loadDashboardNotice(ctx context.Context) *dashboardNotice {
	var value string
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect,
		`SELECT value FROM als_portal_settings WHERE key = ?;`),
		portalSettingDashboardNotice).Scan(&value)
	if err != nil {
		return nil
	}
	var notice dashboardNotice
	if err := json.Unmarshal([]byte(value), &notice); err != nil {
		slog.Error("parse dashboard notice setting failed", "error", err)
		return nil
	}
	return &notice
}

// activeDashboardNotice 返回对用户生效的公告：enabled 且标题、正文非空。
func (r *routes) activeDashboardNotice(ctx context.Context) *dashboardNotice {
	notice := r.loadDashboardNotice(ctx)
	if notice == nil || !notice.Enabled ||
		strings.TrimSpace(notice.Title) == "" || strings.TrimSpace(notice.Body) == "" {
		return nil
	}
	return notice
}

func (r *routes) saveDashboardNotice(ctx context.Context, notice dashboardNotice) error {
	raw, err := json.Marshal(notice)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, db.Rebind(r.sqlDialect, `
		INSERT INTO als_portal_settings(key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP;
	`), portalSettingDashboardNotice, string(raw))
	return err
}

// handleDashboardNotice 返回当前生效公告与该用户的确认状态（登录态）。
func (r *routes) handleDashboardNotice(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	notice := r.activeDashboardNotice(req.Context())
	if notice == nil {
		writeJSON(w, http.StatusOK, map[string]any{"notice": nil})
		return
	}

	acknowledged := false
	var ackedNoticeID string
	err := r.db.QueryRowContext(req.Context(), db.Rebind(r.sqlDialect,
		`SELECT notice_id FROM als_dashboard_notice_acks WHERE user_id = ?;`),
		user.ID).Scan(&ackedNoticeID)
	if err == nil && ackedNoticeID == notice.ID {
		acknowledged = true
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"notice": map[string]any{
			"id":         notice.ID,
			"title":      notice.Title,
			"body":       notice.Body,
			"updated_at": notice.UpdatedAt,
		},
		"acknowledged": acknowledged,
	})
}

// handleDashboardNoticeAck 记录当前用户对当前公告的确认（登录态）。公告已下线
// 时 404，避免确认请求在无公告状态下写出脏 ack。
func (r *routes) handleDashboardNoticeAck(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	notice := r.activeDashboardNotice(req.Context())
	if notice == nil {
		writeError(w, http.StatusNotFound, "no active dashboard notice")
		return
	}
	if _, err := r.db.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `
		INSERT INTO als_dashboard_notice_acks(user_id, notice_id, acked_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE
		SET notice_id = excluded.notice_id, acked_at = CURRENT_TIMESTAMP;
	`), user.ID, notice.ID); err != nil {
		slog.Error("ack dashboard notice failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to acknowledge dashboard notice")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"acknowledged": true})
}

// handleAdminGetDashboardNotice 回显公告全文（含未启用的）供后台编辑。
func (r *routes) handleAdminGetDashboardNotice(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"notice": r.loadDashboardNotice(req.Context())})
}

// handleAdminUpdateDashboardNotice 保存公告。启用时标题、正文必填；enabled
// 省略时保留现值（防止调用方漏字段导致公告静默下线）。保存即以内容哈希为 id
// 落库，改文案自动换 id 触发全量重弹（见文件头注释）。
func (r *routes) handleAdminUpdateDashboardNotice(w http.ResponseWriter, req *http.Request) {
	var payload struct {
		Title   string `json:"title"`
		Body    string `json:"body"`
		Enabled *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	title := strings.TrimSpace(payload.Title)
	body := strings.TrimSpace(payload.Body)

	enabled := false
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	} else if existing := r.loadDashboardNotice(req.Context()); existing != nil {
		enabled = existing.Enabled
	}
	if enabled && (title == "" || body == "") {
		writeError(w, http.StatusBadRequest, "title and body are required when the notice is enabled")
		return
	}

	notice := dashboardNotice{
		ID:        noticeIDFor(title, body),
		Title:     title,
		Body:      body,
		Enabled:   enabled,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := r.saveDashboardNotice(req.Context(), notice); err != nil {
		slog.Error("update dashboard notice failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update dashboard notice")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notice": notice})
}
