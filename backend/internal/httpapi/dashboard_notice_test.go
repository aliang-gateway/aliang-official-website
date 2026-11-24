package httpapi

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newDashboardNoticeRoutes(t *testing.T) (*http.ServeMux, *sql.DB, *routes) {
	t.Helper()
	database := setupTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{SQLDialect: "sqlite"})
	return mux, database, nil
}

// putDashboardNotice 以 admin 身份保存公告，返回响应。
func putDashboardNotice(t *testing.T, mux *http.ServeMux, adminToken, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/admin/dashboard-notice", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// getDashboardNoticeAsUser 以指定身份读取公告。
func getDashboardNoticeAsUser(t *testing.T, mux *http.ServeMux, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/notice", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestAdminDashboardNoticeSaveAndReadback(t *testing.T) {
	mux, database, _ := newDashboardNoticeRoutes(t)
	adminToken := seedUserWithSession(t, database, "notice-admin@example.com", "admin")

	rec := putDashboardNotice(t, mux, adminToken, `{"title":"维护通知","body":"今晚 02:00 维护","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /admin/dashboard-notice: got %d want 200, body=%s", rec.Code, rec.Body.String())
	}

	// admin 读回：含 enabled 字段
	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard-notice", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/dashboard-notice: got %d want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"enabled":true`) || !strings.Contains(rec.Body.String(), "维护通知") {
		t.Fatalf("admin readback missing fields: %s", rec.Body.String())
	}
}

func TestDashboardNoticeUserFlowAckAndRepopOnContentChange(t *testing.T) {
	mux, database, _ := newDashboardNoticeRoutes(t)
	adminToken := seedUserWithSession(t, database, "notice-admin2@example.com", "admin")
	userToken := seedUserWithSession(t, database, "notice-user@example.com", "user")

	if rec := putDashboardNotice(t, mux, adminToken, `{"title":"A","body":"first","enabled":true}`); rec.Code != http.StatusOK {
		t.Fatalf("seed notice: got %d", rec.Code)
	}

	// 未确认 → notice + acknowledged=false
	rec := getDashboardNoticeAsUser(t, mux, userToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET notice: got %d want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"acknowledged":false`) {
		t.Fatalf("expected acknowledged=false, got %s", rec.Body.String())
	}

	// 确认 → acknowledged=true
	req := httptest.NewRequest(http.MethodPost, "/dashboard/notice", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST ack: got %d want 200, body=%s", rec.Code, rec.Body.String())
	}

	rec = getDashboardNoticeAsUser(t, mux, userToken)
	if !strings.Contains(rec.Body.String(), `"acknowledged":true`) {
		t.Fatalf("expected acknowledged=true after ack, got %s", rec.Body.String())
	}

	// 改内容 → id 变化 → 重新弹（acknowledged=false）
	if rec := putDashboardNotice(t, mux, adminToken, `{"title":"A","body":"second","enabled":true}`); rec.Code != http.StatusOK {
		t.Fatalf("update notice: got %d", rec.Code)
	}
	rec = getDashboardNoticeAsUser(t, mux, userToken)
	if !strings.Contains(rec.Body.String(), `"acknowledged":false`) {
		t.Fatalf("expected acknowledged=false after content change, got %s", rec.Body.String())
	}

	// 只切开关不动内容 → id 不变 → 不重弹（此处先 ack 再关再开）
	req = httptest.NewRequest(http.MethodPost, "/dashboard/notice", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("re-ack: got %d", rec.Code)
	}
	if rec := putDashboardNotice(t, mux, adminToken, `{"title":"A","body":"second","enabled":false}`); rec.Code != http.StatusOK {
		t.Fatalf("disable notice: got %d", rec.Code)
	}
	// 禁用后用户侧 notice=null
	rec = getDashboardNoticeAsUser(t, mux, userToken)
	if !strings.Contains(rec.Body.String(), `"notice":null`) {
		t.Fatalf("expected notice=null when disabled, got %s", rec.Body.String())
	}
	// 禁用状态下 ack 应 404
	req = httptest.NewRequest(http.MethodPost, "/dashboard/notice", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("ack on disabled notice: got %d want 404", rec.Code)
	}
	if rec := putDashboardNotice(t, mux, adminToken, `{"title":"A","body":"second","enabled":true}`); rec.Code != http.StatusOK {
		t.Fatalf("re-enable notice: got %d", rec.Code)
	}
	rec = getDashboardNoticeAsUser(t, mux, userToken)
	if !strings.Contains(rec.Body.String(), `"acknowledged":true`) {
		t.Fatalf("enable toggle must not reset ack, got %s", rec.Body.String())
	}
}

func TestDashboardNoticeGuards(t *testing.T) {
	mux, database, _ := newDashboardNoticeRoutes(t)
	adminToken := seedUserWithSession(t, database, "notice-admin3@example.com", "admin")
	userToken := seedUserWithSession(t, database, "notice-user3@example.com", "user")

	// 普通用户不能写公告
	rec := putDashboardNotice(t, mux, userToken, `{"title":"x","body":"y","enabled":true}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin PUT: got %d want 403", rec.Code)
	}

	// 启用但内容缺失 → 400
	rec = putDashboardNotice(t, mux, adminToken, `{"title":"","body":"y","enabled":true}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("enabled without title: got %d want 400", rec.Code)
	}
	rec = putDashboardNotice(t, mux, adminToken, `{"title":"x","body":"","enabled":true}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("enabled without body: got %d want 400", rec.Code)
	}

	// 无公告时用户侧 notice=null，ack 404
	rec = getDashboardNoticeAsUser(t, mux, userToken)
	if !strings.Contains(rec.Body.String(), `"notice":null`) {
		t.Fatalf("expected notice=null with no notice stored, got %s", rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/notice", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("ack with no notice: got %d want 404", rec.Code)
	}
}
