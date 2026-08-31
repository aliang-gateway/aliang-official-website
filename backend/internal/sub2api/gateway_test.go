package sub2api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/proxy"
	"ai-api-portal/backend/internal/sub2apiauth"
)

// --- mocks ---

type mockAuth struct {
	token string
	err   error
}

func (m *mockAuth) GetBearerTokenByUserID(_ context.Context, _ int64) (string, error) {
	return m.token, m.err
}

func (m *mockAuth) UpsertToken(_ context.Context, _ sub2apiauth.UpsertTokenInput) error {
	return nil
}

type mockProxy struct {
	apiKeyResp *proxy.ResponseEnvelope[proxy.APIKey]
	err        error
}

func (m *mockProxy) CreateUserAPIKey(_ context.Context, _ string, _ proxy.CreateUserAPIKeyRequest, _ string) (*proxy.ResponseEnvelope[proxy.APIKey], error) {
	return m.apiKeyResp, m.err
}

type mockResolver struct {
	userID int64
	found  bool
	err    error

	role      string
	roleFound bool
	roleErr   error
}

func (m *mockResolver) FindUserIDBySession(_ context.Context, _ string) (int64, bool, error) {
	return m.userID, m.found, m.err
}

func (m *mockResolver) FindUserRoleByID(_ context.Context, _ int64) (string, bool, error) {
	return m.role, m.roleFound, m.roleErr
}

func (m *mockResolver) EnsureFreshUpstreamAccessToken(_ context.Context, _ int64) error {
	return nil
}

// Since Gateway uses concrete types, we test via the Gateway struct
// with real sub2apiauth.Service (in-memory) or by testing individual methods
// that only depend on the auth interface.

func TestNewGateway(t *testing.T) {
	g := NewGateway(nil, nil)
	if g == nil {
		t.Fatal("expected non-nil gateway")
	}
}

func TestGateway_IsConfigured(t *testing.T) {
	tests := []struct {
		name    string
		gateway *Gateway
		want    bool
	}{
		{"nil gateway", nil, false},
		{"nil deps", NewGateway(nil, nil), false},
		{"only proxy", NewGateway(&proxy.Client{}, nil), false},
		{"only auth", NewGateway(nil, &sub2apiauth.Service{}), false},
		{"both present", NewGateway(&proxy.Client{}, &sub2apiauth.Service{}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.gateway.IsConfigured(); got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGateway_HasUpstreamToken_NilGateway(t *testing.T) {
	var g *Gateway
	ok, err := g.HasUpstreamToken(context.Background(), 1)
	if ok || err != nil {
		t.Errorf("expected (false, nil), got (%v, %v)", ok, err)
	}
}

func TestGateway_CaptureTokens_NilGateway(t *testing.T) {
	var g *Gateway
	err := g.CaptureTokens(context.Background(), sub2apiauth.UpsertTokenInput{})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestGateway_ReplaceAuthHeader_NilGateway(t *testing.T) {
	var g *Gateway
	headers := http.Header{}
	headers.Set("Authorization", "Bearer test")
	err := g.ReplaceAuthHeader(context.Background(), headers, &mockResolver{})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestGateway_ReplaceAuthHeader_NilHeaders(t *testing.T) {
	g := NewGateway(&proxy.Client{}, &sub2apiauth.Service{})
	err := g.ReplaceAuthHeader(context.Background(), nil, &mockResolver{})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestGateway_ReplaceAuthHeader_EmptyAuth(t *testing.T) {
	g := NewGateway(&proxy.Client{}, &sub2apiauth.Service{})
	headers := http.Header{}
	err := g.ReplaceAuthHeader(context.Background(), headers, &mockResolver{})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestGateway_ReplaceAuthHeader_InvalidBearer(t *testing.T) {
	g := NewGateway(&proxy.Client{}, &sub2apiauth.Service{})
	headers := http.Header{}
	headers.Set("Authorization", "Basic abc123")
	err := g.ReplaceAuthHeader(context.Background(), headers, &mockResolver{})
	if err != nil {
		t.Errorf("expected nil error for invalid bearer, got %v", err)
	}
}

func TestGateway_ReplaceAuthHeader_UserNotFound(t *testing.T) {
	g := NewGateway(&proxy.Client{}, &sub2apiauth.Service{})
	headers := http.Header{}
	headers.Set("Authorization", "Bearer session-token")
	resolver := &mockResolver{userID: 0, found: false}
	err := g.ReplaceAuthHeader(context.Background(), headers, resolver)
	if err != nil {
		t.Errorf("expected nil error for user not found, got %v", err)
	}
}

func TestGateway_ReplaceAuthHeader_ResolverError(t *testing.T) {
	g := NewGateway(&proxy.Client{}, &sub2apiauth.Service{})
	headers := http.Header{}
	headers.Set("Authorization", "Bearer session-token")
	resolver := &mockResolver{err: errors.New("db down")}
	err := g.ReplaceAuthHeader(context.Background(), headers, resolver)
	if err == nil {
		t.Error("expected error from resolver")
	}
}

func TestGateway_EnsureUserKeyInGroup_NilDeps(t *testing.T) {
	g := NewGateway(nil, &sub2apiauth.Service{})
	err := g.EnsureUserKeyInGroup(context.Background(), 1, 10, "parent-key")
	if err != nil {
		t.Errorf("expected nil for nil proxy, got %v", err)
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"Bearer abc123", "abc123", false},
		{"Bearer   abc123   ", "abc123", false},
		{"bearer abc123", "", true}, // case-sensitive
		{"Basic abc123", "", true},
		{"", "", true},
		{"Bearer ", "", true},
		{"Bearer", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := extractBearerToken(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractBearerToken(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("extractBearerToken(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSelectDefaultKeyGroups(t *testing.T) {
	groups := []proxy.AvailableGroup{
		{ID: 3, Platform: "anthropic", Status: "active", SubscriptionType: ""},
		{ID: 4, Platform: "Anthropic", Status: "active", SubscriptionType: ""},          // 平台大小写去重 → 跳过
		{ID: 7, Platform: "openai", Status: "active", SubscriptionType: "subscription"}, // 订阅组 → 跳过
		{ID: 8, Platform: "openai", Status: "active", SubscriptionType: ""},             // openai 第一个标准组
		{ID: 12, Platform: "openai", Status: "active", SubscriptionType: ""},            // 同平台第二个标准组 → 跳过（first-wins）
		{ID: 9, Platform: "gemini", Status: "inactive", SubscriptionType: ""},           // 非active → 跳过
		{ID: 0, Platform: "gemini", Status: "active", SubscriptionType: ""},             // 非法ID → 跳过
		{ID: 5, Platform: "mistral", Status: "ACTIVE", SubscriptionType: ""},            // 大写状态容忍 → 选中
		{ID: 6, Platform: "cohere", Status: "  active  ", SubscriptionType: ""},         // 状态首尾空白容忍 → 选中
		{ID: 13, Platform: "qwen", Status: "active", SubscriptionType: "Subscription"},  // 大小写不敏感订阅判断 → 跳过
		{ID: -1, Platform: "neg", Status: "active", SubscriptionType: ""},               // 负数ID → 跳过
		{ID: 11, Platform: "", Status: "active", SubscriptionType: ""},                  // 空平台独立桶
	}
	got := selectDefaultKeyGroups(groups)
	want := []int64{3, 8, 5, 6, 11}
	if len(got) != len(want) {
		t.Fatalf("selectDefaultKeyGroups = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("selectDefaultKeyGroups = %v, want %v", got, want)
		}
	}
}

func TestSelectDefaultKeyGroups_Empty(t *testing.T) {
	if got := selectDefaultKeyGroups(nil); len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

// --- EnsureDefaultUserKeys ---

func strPtr(s string) *string        { return &s }
func timePtr(v time.Time) *time.Time { return &v }

// newEnsureTestGateway builds a Gateway backed by an httptest upstream and a
// real sub2apiauth service with user 1 having a stored token. The recorder
// captures upstream requests for assertions.
type ensureUpstreamRecorder struct {
	mu        sync.Mutex
	requests  []string // "METHOD PATH IDEMKEY" per request
	listBody  string   // body for GET /api/v1/groups/available
	keysBody  string   // body for GET /api/v1/api-keys (raw JSON array form)
	createErr string   // when non-empty, respond with this status code text on create
}

func newEnsureTestGateway(t *testing.T, rec *ensureUpstreamRecorder) *Gateway {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idem := r.Header.Get("Idempotency-Key")
		rec.mu.Lock()
		rec.requests = append(rec.requests, r.Method+" "+r.URL.Path+" "+idem)
		rec.mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/groups/available":
			_, _ = w.Write([]byte(rec.listBody))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/api-keys"):
			// 裸数组形态：ListResponseEnvelope 的自定义反序列化明确支持 `[` 开头。
			keysBody := rec.keysBody
			if keysBody == "" {
				keysBody = "[]"
			}
			_, _ = w.Write([]byte(keysBody))
		case r.Method == http.MethodPost && (r.URL.Path == "/api/v1/api-keys" || r.URL.Path == "/api/v1/keys"):
			if rec.createErr != "" {
				http.Error(w, rec.createErr, http.StatusConflict)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"id":10,"key":"sk-x","name":"auto-key","group_id":3,"status":"active"}}`))
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := proxy.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("proxy.NewClient: %v", err)
	}
	database := setupEnsureTestDB(t)
	createEnsureTestUser(t, context.Background(), database, "u1@test.local", "User One", "user")
	authSvc := sub2apiauth.NewService(database)
	err = authSvc.UpsertToken(context.Background(), sub2apiauth.UpsertTokenInput{
		UserID:          1,
		AccessToken:     "access-1",
		RefreshToken:    strPtr("refresh-1"),
		AccessExpiresAt: timePtr(time.Now().Add(time.Hour)),
	})
	if err != nil {
		t.Fatalf("UpsertToken: %v", err)
	}
	return NewGateway(client, authSvc)
}

const ensureAvailableGroupsBody = `{"data":[` +
	`{"id":3,"name":"Claude Basic","platform":"anthropic","status":"active","subscription_type":""},` +
	`{"id":7,"name":"GPT Sub","platform":"openai","status":"active","subscription_type":"subscription"},` +
	`{"id":8,"name":"GPT Basic","platform":"openai","status":"active","subscription_type":""}]}`

func TestEnsureDefaultUserKeys_CreatesOneKeyPerPlatform(t *testing.T) {
	rec := &ensureUpstreamRecorder{listBody: ensureAvailableGroupsBody}
	g := newEnsureTestGateway(t, rec)

	result, err := g.EnsureDefaultUserKeys(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnsureDefaultUserKeys: %v", err)
	}
	if result.Ensured != 2 || len(result.CreatedGroups) != 2 {
		t.Fatalf("result = %+v, want ensured=2 created=2 (groups 3 and 8)", result)
	}

	// 幂等键确定性：同一 (user, group) 永远同一键。
	wantKeys := map[string]bool{"auto-ensure:u1:g3": false, "auto-ensure:u1:g8": false}
	for _, req := range rec.requests {
		for key := range wantKeys {
			if strings.Contains(req, key) {
				wantKeys[key] = true
			}
		}
	}
	for key, seen := range wantKeys {
		if !seen {
			t.Errorf("expected upstream create with Idempotency-Key %q, requests=%v", key, rec.requests)
		}
	}
}

func TestEnsureDefaultUserKeys_IdempotentSecondRun(t *testing.T) {
	groupsBody := `{"data":[{"id":3,"name":"Claude Basic","platform":"anthropic","status":"active","subscription_type":""}]}`
	rec := &ensureUpstreamRecorder{listBody: groupsBody}
	g := newEnsureTestGateway(t, rec)

	if _, err := g.EnsureDefaultUserKeys(context.Background(), 1); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// 第二轮：组列表不变，但上游 api-keys 列表已含该组 auto-key → 不应再发起 create。
	rec.requests = nil
	rec.keysBody = `[{"id":10,"name":"auto-key","group_id":3,"status":"active"}]`
	result, err := g.EnsureDefaultUserKeys(context.Background(), 1)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if result.Ensured != 1 || len(result.CreatedGroups) != 0 {
		t.Fatalf("second run result = %+v, want ensured=1 created=0", result)
	}
	for _, req := range rec.requests {
		if strings.HasPrefix(req, "POST") {
			t.Errorf("second run must not create, got %q", req)
		}
	}
}

func TestEnsureDefaultUserKeys_ConflictTreatedAsEnsured(t *testing.T) {
	rec := &ensureUpstreamRecorder{
		listBody:  ensureAvailableGroupsBody,
		createErr: "conflict",
	}
	g := newEnsureTestGateway(t, rec)

	result, err := g.EnsureDefaultUserKeys(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnsureDefaultUserKeys: %v", err)
	}
	if result.Ensured != 2 || len(result.CreatedGroups) != 0 {
		t.Fatalf("result = %+v, want 409 counted as ensured-but-not-created", result)
	}
}

func TestEnsureDefaultUserKeys_NotConfigured(t *testing.T) {
	g := NewGateway(nil, nil)
	if _, err := g.EnsureDefaultUserKeys(context.Background(), 1); err == nil {
		t.Error("expected error when gateway not configured")
	}
}

func TestEnsureDefaultUserKeys_UpstreamDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer srv.Close()
	client, err := proxy.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("proxy.NewClient: %v", err)
	}
	database := setupEnsureTestDB(t)
	createEnsureTestUser(t, context.Background(), database, "u2@test.local", "User Two", "user")
	authSvc := sub2apiauth.NewService(database)
	if err := authSvc.UpsertToken(context.Background(), sub2apiauth.UpsertTokenInput{
		UserID:          1,
		AccessToken:     "access-1",
		RefreshToken:    strPtr("refresh-1"),
		AccessExpiresAt: timePtr(time.Now().Add(time.Hour)),
	}); err != nil {
		t.Fatalf("UpsertToken: %v", err)
	}
	g := NewGateway(client, authSvc)
	if _, err := g.EnsureDefaultUserKeys(context.Background(), 1); err == nil {
		t.Error("expected error when upstream unavailable")
	}
}

// setupEnsureTestDB / createEnsureTestUser / ensureTestDialect（含其依赖的
// ensureTestSchemaName）逐行复制自 backend/internal/sub2apiauth/service_test.go
// 的 setupTestDB / createUser / testDialect / testSchemaName 后改名 —— 建库与
// 迁移语句保持一致，禁止凭空编写。

func setupEnsureTestDB(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()
	dialect := ensureTestDialect()
	if dialect == "postgres" {
		dsn := strings.TrimSpace(os.Getenv("DB_DSN"))
		if dsn == "" {
			t.Skip("DB_DSN is required when DB_DRIVER=postgres")
		}

		bootstrap, err := db.Open(ctx, "postgres", dsn)
		if err != nil {
			t.Fatalf("Open bootstrap postgres error = %v", err)
		}

		schema := ensureTestSchemaName(t)
		if _, err := bootstrap.ExecContext(ctx, fmt.Sprintf(`CREATE SCHEMA "%s";`, schema)); err != nil {
			_ = bootstrap.Close()
			t.Fatalf("create schema %s error = %v", schema, err)
		}

		database, err := db.Open(ctx, "postgres", dsn)
		if err != nil {
			_, _ = bootstrap.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE;`, schema))
			_ = bootstrap.Close()
			t.Fatalf("Open postgres error = %v", err)
		}
		database.SetMaxOpenConns(1)
		database.SetMaxIdleConns(1)
		if _, err := database.ExecContext(ctx, fmt.Sprintf(`SET search_path TO "%s";`, schema)); err != nil {
			_ = database.Close()
			_, _ = bootstrap.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE;`, schema))
			_ = bootstrap.Close()
			t.Fatalf("set search_path error = %v", err)
		}
		if err := db.ApplyMigrations(ctx, database, "postgres"); err != nil {
			_ = database.Close()
			_, _ = bootstrap.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE;`, schema))
			_ = bootstrap.Close()
			t.Fatalf("ApplyMigrations() error = %v", err)
		}
		t.Cleanup(func() {
			_ = database.Close()
			_, _ = bootstrap.ExecContext(context.Background(), fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE;`, schema))
			_ = bootstrap.Close()
		})
		return database
	}

	dbFile := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(ctx, "sqlite", dbFile)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := db.ApplyMigrations(ctx, database, "sqlite"); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}

	return database
}

func createEnsureTestUser(t *testing.T, ctx context.Context, database *sql.DB, email, name, role string) int64 {
	t.Helper()

	id, err := db.InsertID(ctx, ensureTestDialect(), database, `INSERT INTO als_users(email, name, role) VALUES (?, ?, ?);`, "id", email, name, role)
	if err != nil {
		t.Fatalf("insert user error = %v", err)
	}

	return id
}

func ensureTestDialect() string {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("DB_DRIVER")), "postgres") {
		return "postgres"
	}
	return "sqlite"
}

func ensureTestSchemaName(t *testing.T) string {
	name := strings.ToLower(t.Name())
	var builder strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	return fmt.Sprintf("test_%s_%d", builder.String(), time.Now().UnixNano())
}
