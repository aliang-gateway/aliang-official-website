package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"ai-api-portal/backend/internal/proxy"
	"ai-api-portal/backend/internal/user"
)

// capturingMailSender 同 user 包：捕获发信供断言。
type MailCall struct {
	ToEmail string
	Subject string
	Body    string
}

type capturingMailSender struct {
	mu    sync.Mutex
	calls []MailCall
}

func (s *capturingMailSender) Send(_ context.Context, toEmail, subject, body string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, MailCall{ToEmail: toEmail, Subject: subject, Body: body})
	return nil
}

func (s *capturingMailSender) lastBody() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return ""
	}
	return s.calls[len(s.calls)-1].Body
}

func newEmailVerificationRoutes(t *testing.T, opts RoutesOptions) (*http.ServeMux, *sql.DB, *capturingMailSender) {
	t.Helper()

	// userSvc 与 routes 必须共享同一个 DB：发码写入的验证码表由守卫的
	// verifyEmailCode（routes 侧 userSvc）读取。
	database := setupTestDB(t)
	mailSender := &capturingMailSender{}
	opts.UserService = user.NewServiceWithOptions(database, user.ServiceOptions{MailSender: mailSender, SQLDialect: "sqlite"})
	opts.SmtpConfigured = true

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, opts)
	return mux, database, mailSender
}

// enableEmailVerification 直接写 settings 表打开开关（绕过 admin API）。
func enableEmailVerification(t *testing.T, database *sql.DB) {
	t.Helper()
	if _, err := database.Exec(`
		INSERT INTO als_portal_settings(key, value, updated_at)
		VALUES ('register_email_verification', 'true', CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = 'true', updated_at = CURRENT_TIMESTAMP;
	`); err != nil {
		t.Fatalf("enable email verification: %v", err)
	}
}

func postJSON(t *testing.T, mux *http.ServeMux, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestRegisterSendCode_IssuesAndMailsCode(t *testing.T) {
	t.Parallel()

	mux, database, mailSender := newEmailVerificationRoutes(t, RoutesOptions{})
	enableEmailVerification(t, database)

	rec := postJSON(t, mux, "/auth/register/send-code", `{"email":"newuser@example.com"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := mailSender.lastBody(); !strings.Contains(body, "code") {
		t.Fatalf("expected verification email, got %q", body)
	}
}

func TestRegisterSendCode_PhysicallyOffWithoutSMTP(t *testing.T) {
	t.Parallel()

	// SMTP 未配置（生产现状）：即使 settings 开关为 true，功能也必须物理
	// 失效——否则发码 200 但信件被 noop 吞掉，注册进入死锁。
	database := setupTestDB(t)
	mailSender := &capturingMailSender{}
	userSvc := user.NewServiceWithOptions(database, user.ServiceOptions{MailSender: mailSender, SQLDialect: "sqlite"})
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{UserService: userSvc, SQLDialect: "sqlite", SmtpConfigured: false})
	enableEmailVerification(t, database)

	req := httptest.NewRequest(http.MethodGet, "/auth/register/options", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), `"email_verification":true`) {
		t.Fatalf("without SMTP the switch must report off: %s", rec.Body.String())
	}
	if rec := postJSON(t, mux, "/auth/register/send-code", `{"email":"deadlock@example.com"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 without SMTP, got %d", rec.Code)
	}
}

func TestRegisterSendCode_DisabledWhenSwitchOff(t *testing.T) {
	t.Parallel()

	// default=false 且 DB 无记录 → 开关关 → 404
	mux, database, _ := newEmailVerificationRoutes(t, RoutesOptions{RegisterEmailVerificationDefault: false})
	_ = database

	rec := postJSON(t, mux, "/auth/register/send-code", `{"email":"newuser@example.com"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when the switch is off, got %d", rec.Code)
	}
}

func TestRegisterSendCode_RateLimitedPerIP(t *testing.T) {
	t.Parallel()

	mux, database, _ := newEmailVerificationRoutes(t, RoutesOptions{})
	enableEmailVerification(t, database)
	// per-IP 3 次/10min：前三次（各不同邮箱，避开每邮箱 2 次上限）都应成功
	for i, email := range []string{"rl-a@example.com", "rl-b@example.com", "rl-c@example.com"} {
		if rec := postJSON(t, mux, "/auth/register/send-code", `{"email":"`+email+`"}`); rec.Code != http.StatusOK {
			t.Fatalf("send %s (#%d): expected 200, got %d", email, i+1, rec.Code)
		}
	}
	// 同一 IP 第 4 次（换新邮箱）触发 per-IP 上限——证明限制与邮箱无关
	if rec := postJSON(t, mux, "/auth/register/send-code", `{"email":"rl-d@example.com"}`); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 once per-IP limit is exceeded, got %d", rec.Code)
	}
}

func TestRegisterOptions_ExposesSwitch(t *testing.T) {
	t.Parallel()

	mux, database, _ := newEmailVerificationRoutes(t, RoutesOptions{})
	enableEmailVerification(t, database)
	req := httptest.NewRequest(http.MethodGet, "/auth/register/options", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"email_verification":true`) {
		t.Fatalf("expected email_verification=true, got %s", rec.Body.String())
	}
}

func TestAdminRegisterSettings_ToggleFlow(t *testing.T) {
	t.Parallel()

	mux, database, _ := newEmailVerificationRoutes(t, RoutesOptions{RegisterEmailVerificationDefault: false})
	adminToken := seedUserWithSession(t, database, "settings-admin@example.com", "admin")

	// 读取初始态（default false）
	req := httptest.NewRequest(http.MethodGet, "/admin/register-settings", nil)
	setBearerAuth(req, adminToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"email_verification_enabled":false`) {
		t.Fatalf("initial settings mismatch: %d %s", rec.Code, rec.Body.String())
	}

	// 开启 → 运行时生效（options 端点翻转）
	req = httptest.NewRequest(http.MethodPut, "/admin/register-settings", strings.NewReader(`{"email_verification_enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	setBearerAuth(req, adminToken)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("enable failed: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/auth/register/options", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"email_verification":true`) {
		t.Fatalf("runtime switch did not flip: %s", rec.Body.String())
	}

	// 关闭
	req = httptest.NewRequest(http.MethodPut, "/admin/register-settings", strings.NewReader(`{"email_verification_enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	setBearerAuth(req, adminToken)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("disable failed: %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/auth/register/options", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"email_verification":false`) {
		t.Fatalf("runtime switch did not flip back: %s", rec.Body.String())
	}
}

func TestAdminRegisterSettings_NonAdminRejected(t *testing.T) {
	t.Parallel()

	mux, database, _ := newEmailVerificationRoutes(t, RoutesOptions{})
	userToken := seedUserWithSession(t, database, "settings-pleb@example.com", "user")

	req := httptest.NewRequest(http.MethodPut, "/admin/register-settings", strings.NewReader(`{"email_verification_enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	setBearerAuth(req, userToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for plain user, got %d", rec.Code)
	}
}

// newEmailVerificationGuardRoutes：带上游 mock 的完整链路（注册透传可达）。
func newEmailVerificationGuardRoutes(t *testing.T) (*http.ServeMux, *sql.DB, *capturingMailSender) {
	t.Helper()

	database := setupTestDB(t)
	mailSender := &capturingMailSender{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/register" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"up-at","refresh_token":"up-rt","role":"user","user":{"id":1,"email":"guard@example.com","role":"user"}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{}}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}
	userSvc := user.NewServiceWithOptions(database, user.ServiceOptions{MailSender: mailSender, SQLDialect: "sqlite"})

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{
		SQLDialect:                       "sqlite",
		ProxyClient:                      proxyClient,
		UserService:                      userSvc,
		RegisterEmailVerificationDefault: true,
		SmtpConfigured:                   true,
	})
	return mux, database, mailSender
}

func TestRegisterGuardRejectsMissingEmailCode(t *testing.T) {
	t.Parallel()

	// 邮箱验证开启（default true）+ 发码 → 注册不带 email_code → 403；
	// 带正确码 → 透传上游。
	mux, database, mailSender := newEmailVerificationGuardRoutes(t)
	enableEmailVerification(t, database)

	// 发码
	rec := postJSON(t, mux, "/auth/register/send-code", `{"email":"guard@example.com"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("send-code failed: %d", rec.Code)
	}
	body := mailSender.lastBody()
	code := extractCodeFromMailBody(t, body)

	// 无码注册 → 403（会打到上游 404？本 harness 无上游 proxy —— RoutesOptions{}
	// 无 ProxyClient → handleAuthPassthrough 500。改断言：无码时应在触达上游前被拒，
	// 即响应不是 proxy 未配置错误。
	rec = postJSON(t, mux, "/auth/register", `{"email":"guard@example.com","password":"pw123456","name":"g"}`)
	if rec.Code == http.StatusInternalServerError && strings.Contains(rec.Body.String(), "proxy is not configured") {
		t.Fatalf("missing email code must be rejected before upstream: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing email code, got %d: %s", rec.Code, rec.Body.String())
	}

	// 错码 → 403
	rec = postJSON(t, mux, "/auth/register", `{"email":"guard@example.com","password":"pw123456","name":"g","email_code":"WRONGWRONG"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for wrong email code, got %d", rec.Code)
	}

	// 正确码 → 守卫放行 → 透传上游 → 注册成功
	rec = postJSON(t, mux, "/auth/register", `{"email":"guard@example.com","password":"pw123456","name":"g","email_code":"`+code+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("correct code should register successfully, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "session_token") {
		t.Fatalf("expected a session in the register response: %s", rec.Body.String())
	}
	_ = database
}

// extractCodeFromMailBody 从 "Your verification code is: XXXX" 里取码。
func extractCodeFromMailBody(t *testing.T, body string) string {
	t.Helper()
	marker := "code is: "
	idx := strings.Index(body, marker)
	if idx < 0 {
		t.Fatalf("mail body missing code marker: %q", body)
	}
	rest := body[idx+len(marker):]
	end := strings.IndexAny(rest, "\r\n ")
	if end < 0 {
		end = len(rest)
	}
	return strings.TrimSpace(rest[:end])
}
