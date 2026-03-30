package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"testing"

	"ai-api-portal/backend/internal/proxy"
)

func TestAuthLoginPassthroughStoresSub2APITokensByEmail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	userID := createUser(t, ctx, database, "token-login@example.com", "Token Login", "user")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/auth/login" {
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"up-at-1","refresh_token":"up-rt-1","user":{"id":7001,"email":"token-login@example.com"}}}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	m := http.NewServeMux()
	RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte(`{"email":"token-login@example.com","password":"secret"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]any
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode login response payload: %v", err)
	}
	if got := payload["session_token"]; got == nil || got == "" {
		t.Fatalf("expected root session_token to be injected, got %#v", got)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested data object, got %#v", payload["data"])
	}
	if data["access_token"] != "up-at-1" {
		t.Fatalf("expected access_token up-at-1, got %#v", data["access_token"])
	}
	if data["refresh_token"] != "up-rt-1" {
		t.Fatalf("expected refresh_token up-rt-1, got %#v", data["refresh_token"])
	}
	if got := data["session_token"]; got == nil || got == "" {
		t.Fatalf("expected nested session_token to be injected, got %#v", got)
	}
	if got := rec.Header().Get("Content-Length"); got != "" && got != strconv.Itoa(rec.Body.Len()) {
		t.Fatalf("expected Content-Length %d, got %q", rec.Body.Len(), got)
	}

	var (
		storedUserID   int64
		storedAccess   string
		storedRefresh  sql.NullString
		storedUpstream sql.NullInt64
	)
	err = database.QueryRowContext(ctx, `
		SELECT user_id, access_token, refresh_token, upstream_user_id
		FROM als_sub2api_auth_tokens
		WHERE user_id = ?;
	`, userID).Scan(&storedUserID, &storedAccess, &storedRefresh, &storedUpstream)
	if err != nil {
		t.Fatalf("query stored sub2api tokens: %v", err)
	}
	if storedUserID != userID {
		t.Fatalf("expected stored user id %d, got %d", userID, storedUserID)
	}
	if storedAccess != "up-at-1" {
		t.Fatalf("expected stored access token up-at-1, got %q", storedAccess)
	}
	if !storedRefresh.Valid || storedRefresh.String != "up-rt-1" {
		t.Fatalf("expected stored refresh token up-rt-1, got %+v", storedRefresh)
	}
	if !storedUpstream.Valid || storedUpstream.Int64 != 7001 {
		t.Fatalf("expected stored upstream user id 7001, got %+v", storedUpstream)
	}

	var sessionCount int64
	err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_sessions WHERE user_id = ?;`, userID).Scan(&sessionCount)
	if err != nil {
		t.Fatalf("count local als_sessions: %v", err)
	}
	if sessionCount != 1 {
		t.Fatalf("expected one local session for login, got %d", sessionCount)
	}
}

func TestAuthRefreshPassthroughUpdatesStoredSub2APITokens(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	userID := createUser(t, ctx, database, "token-refresh@example.com", "Token Refresh", "user")

	_, err := database.ExecContext(ctx, `
		INSERT INTO als_sub2api_auth_tokens(user_id, access_token, refresh_token)
		VALUES (?, ?, ?);
	`, userID, "old-access", "old-refresh")
	if err != nil {
		t.Fatalf("seed als_sub2api_auth_tokens row: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/auth/refresh" {
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh"}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	m := http.NewServeMux()
	RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader([]byte(`{"refresh_token":"old-refresh"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != `{"access_token":"new-access","refresh_token":"new-refresh"}` {
		t.Fatalf("expected unchanged passthrough body, got %q", rec.Body.String())
	}

	var (
		storedAccess  string
		storedRefresh sql.NullString
	)
	err = database.QueryRowContext(ctx, `
		SELECT access_token, refresh_token
		FROM als_sub2api_auth_tokens
		WHERE user_id = ?;
	`, userID).Scan(&storedAccess, &storedRefresh)
	if err != nil {
		t.Fatalf("query updated sub2api tokens: %v", err)
	}
	if storedAccess != "new-access" {
		t.Fatalf("expected updated access token new-access, got %q", storedAccess)
	}
	if !storedRefresh.Valid || storedRefresh.String != "new-refresh" {
		t.Fatalf("expected updated refresh token new-refresh, got %+v", storedRefresh)
	}
}

func TestAuthMePassthroughSwapsLocalSessionForStoredUpstreamToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)

	var seenAuthorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/auth/me" {
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
		seenAuthorization = req.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":9999,"email":"from-upstream@example.com"}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	m := http.NewServeMux()
	RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

	userID, localSessionToken := createUserViaAPI(t, m, "passthrough-authme@example.com", "Auth Me User", "user", "")

	_, err = database.ExecContext(ctx, `
		INSERT INTO als_sub2api_auth_tokens(user_id, access_token, refresh_token)
		VALUES (?, ?, ?);
	`, userID, "stored-upstream-access", "stored-upstream-refresh")
	if err != nil {
		t.Fatalf("seed als_sub2api_auth_tokens row: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+localSessionToken)
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if seenAuthorization != "Bearer stored-upstream-access" {
		t.Fatalf("expected upstream Authorization to use stored upstream token, got %q", seenAuthorization)
	}
}

func TestAuthMeReturnsLocalProfileWhenNoUpstreamTokenExists(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)

	upstreamHits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		upstreamHits++
		t.Fatalf("unexpected upstream request for %s", req.URL.Path)
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	m := http.NewServeMux()
	RegisterRoutesWithOptions(m, database, RoutesOptions{
		AdminBootstrapSecret: "test-admin-secret",
		ProxyClient:          proxyClient,
	})

	_, adminSessionToken := createUserViaAPI(t, m, "local-admin@example.com", "Local Admin", "admin", "test-admin-secret")

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+adminSessionToken)
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if upstreamHits != 0 {
		t.Fatalf("expected no upstream calls, got %d", upstreamHits)
	}

	var payload struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode local auth/me response: %v", err)
	}
	if payload.Email != "local-admin@example.com" || payload.Name != "Local Admin" || payload.Role != "admin" {
		t.Fatalf("unexpected local auth/me payload: %+v", payload)
	}
}

func TestDashboardPassthroughSwapsLocalSessionForStoredUpstreamToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)

	var seenAuthorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/usage/dashboard/stats" {
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
		seenAuthorization = req.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"source":"dashboard-upstream"}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	m := http.NewServeMux()
	RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

	userID, localSessionToken := createUserViaAPI(t, m, "dashboard-auth@example.com", "Dashboard User", "user", "")

	_, err = database.ExecContext(ctx, `
		INSERT INTO als_sub2api_auth_tokens(user_id, access_token, refresh_token)
		VALUES (?, ?, ?);
	`, userID, "dashboard-upstream-access", "dashboard-upstream-refresh")
	if err != nil {
		t.Fatalf("seed als_sub2api_auth_tokens row: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/home", nil)
	req.Header.Set("Authorization", "Bearer "+localSessionToken)
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}
	if seenAuthorization != "Bearer dashboard-upstream-access" {
		t.Fatalf("expected upstream Authorization to use stored upstream token, got %q", seenAuthorization)
	}
}

func TestAuthPassthroughMissingLocalUserDoesNotFail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/auth/login" {
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"missing-user-access","refresh_token":"missing-user-refresh","user":{"email":"unknown@example.com"}}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	m := http.NewServeMux()
	RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte(`{"email":"unknown@example.com","password":"secret"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]any
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode login response payload: %v", err)
	}
	if got := payload["session_token"]; got == nil || got == "" {
		t.Fatalf("expected root session_token to be injected, got %#v", got)
	}
	if payload["access_token"] != "missing-user-access" {
		t.Fatalf("expected access_token missing-user-access, got %#v", payload["access_token"])
	}
	if payload["refresh_token"] != "missing-user-refresh" {
		t.Fatalf("expected refresh_token missing-user-refresh, got %#v", payload["refresh_token"])
	}
	userPayload, ok := payload["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user payload, got %#v", payload["user"])
	}
	if userPayload["email"] != "unknown@example.com" {
		t.Fatalf("expected upstream email unknown@example.com, got %#v", userPayload["email"])
	}

	var count int64
	err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_sub2api_auth_tokens;`).Scan(&count)
	if err != nil {
		t.Fatalf("count als_sub2api_auth_tokens rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one stored token row for auto-created local user, got %d", count)
	}

	var sessionCount int64
	err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_sessions;`).Scan(&sessionCount)
	if err != nil {
		t.Fatalf("count als_sessions rows: %v", err)
	}
	if sessionCount != 1 {
		t.Fatalf("expected one local session for auto-created user, got %d", sessionCount)
	}
}

func TestAuthPassthroughRoutesForwardMethodPathAndBody(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		method       string
		routePath    string
		upstreamPath string
		body         string
		query        string
	}{
		{
			name:         "register",
			method:       http.MethodPost,
			routePath:    "/auth/register",
			upstreamPath: "/api/v1/auth/register",
			body:         `{"email":"new@example.com","password":"secret"}`,
		},
		{
			name:         "login",
			method:       http.MethodPost,
			routePath:    "/auth/login",
			upstreamPath: "/api/v1/auth/login",
			body:         `{"email":"new@example.com","password":"secret"}`,
		},
		{
			name:         "me",
			method:       http.MethodGet,
			routePath:    "/auth/me",
			upstreamPath: "/api/v1/auth/me",
			query:        "include=profile",
		},
		{
			name:         "refresh",
			method:       http.MethodPost,
			routePath:    "/auth/refresh",
			upstreamPath: "/api/v1/auth/refresh",
			body:         `{"refresh_token":"rt_123"}`,
		},
		{
			name:         "logout",
			method:       http.MethodPost,
			routePath:    "/auth/logout",
			upstreamPath: "/api/v1/auth/logout",
			body:         `{"all_devices":true}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			database := setupTestDB(t)

			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != tc.method {
					t.Fatalf("expected method %s, got %s", tc.method, req.Method)
				}
				if req.URL.Path != tc.upstreamPath {
					t.Fatalf("expected upstream path %s, got %s", tc.upstreamPath, req.URL.Path)
				}
				expectedQuery, err := url.ParseQuery(tc.query)
				if err != nil {
					t.Fatalf("parse expected query: %v", err)
				}
				actualQuery, err := url.ParseQuery(req.URL.RawQuery)
				if err != nil {
					t.Fatalf("parse actual query: %v", err)
				}
				if !reflect.DeepEqual(actualQuery, expectedQuery) {
					t.Fatalf("expected query values %v, got %v", expectedQuery, actualQuery)
				}

				bodyBytes, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read upstream request body: %v", err)
				}
				if string(bodyBytes) != tc.body {
					t.Fatalf("expected body %q, got %q", tc.body, string(bodyBytes))
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"source":"upstream"}`))
			}))
			t.Cleanup(upstream.Close)

			proxyClient, err := proxy.NewClient(upstream.URL)
			if err != nil {
				t.Fatalf("create proxy client: %v", err)
			}

			m := http.NewServeMux()
			RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

			path := tc.routePath
			if tc.query != "" {
				path = path + "?" + tc.query
			}
			req := httptest.NewRequest(tc.method, path, bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			m.ServeHTTP(rec, req)

			if rec.Code != http.StatusAccepted {
				t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
			}
			if rec.Body.String() != `{"source":"upstream"}` {
				t.Fatalf("expected passthrough body %q, got %q", `{"source":"upstream"}`, rec.Body.String())
			}
		})
	}
}

func TestAuthPassthroughForwardsAuthorizationAndStripsXUserID(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if got := req.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Fatalf("expected Authorization to be forwarded, got %q", got)
		}
		if got := req.Header.Get("X-User-Id"); got != "" {
			t.Fatalf("expected X-User-Id to be stripped, got %q", got)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	m := http.NewServeMux()
	RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	req.Header.Set("X-User-Id", "999")
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != `{"ok":true}` {
		t.Fatalf("expected passthrough body %q, got %q", `{"ok":true}`, rec.Body.String())
	}
}

func TestAuthPassthroughCopiesUpstreamStatusBodyAndHeaders(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="sub2api"`)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid token"}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	m := http.NewServeMux()
	RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if rec.Body.String() != `{"error":"invalid token"}` {
		t.Fatalf("expected passthrough body %q, got %q", `{"error":"invalid token"}`, rec.Body.String())
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != `Bearer realm="sub2api"` {
		t.Fatalf("expected passthrough WWW-Authenticate, got %q", got)
	}
}

func TestAuthPassthroughReturnsBadGatewayWhenUpstreamUnavailable(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)

	proxyClient, err := proxy.NewClientWithHTTPClient("http://127.0.0.1:1", &http.Client{})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	m := http.NewServeMux()
	RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader([]byte(`{"refresh_token":"rt_123"}`)))
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, rec.Code)
	}
	if rec.Body.String() == "" {
		t.Fatalf("expected non-empty error body")
	}
}

func TestAuthMeUsesPassthroughWhenProxyConfigured(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	userID := createUser(t, ctx, database, "passthrough-me@example.com", "Local User", "user")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/auth/me" {
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":9999,"email":"from-upstream@example.com"}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	m := http.NewServeMux()
	RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

	req := makeAuthenticatedRequest(t, ctx, database, http.MethodGet, "/user/me", nil, userID)
	userMeRec := httptest.NewRecorder()
	m.ServeHTTP(userMeRec, req)
	if userMeRec.Code != http.StatusOK {
		t.Fatalf("expected local /user/me status %d, got %d", http.StatusOK, userMeRec.Code)
	}

	reqAuthMe := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	reqAuthMe.Header.Set("Authorization", "Bearer abc")
	authMeRec := httptest.NewRecorder()
	m.ServeHTTP(authMeRec, reqAuthMe)

	if authMeRec.Code != http.StatusOK {
		t.Fatalf("expected passthrough /auth/me status %d, got %d", http.StatusOK, authMeRec.Code)
	}
	if authMeRec.Body.String() != `{"id":9999,"email":"from-upstream@example.com"}` {
		t.Fatalf("expected upstream /auth/me body, got %q", authMeRec.Body.String())
	}
}

func TestDashboardPassthroughRoutesForwardMethodPathAndQuery(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		routePath    string
		upstreamPath string
		query        string
	}{
		{
			name:         "dashboard home",
			routePath:    "/dashboard/home",
			upstreamPath: "/api/v1/usage/dashboard/stats",
		},
		{
			name:         "dashboard details",
			routePath:    "/dashboard/details",
			upstreamPath: "/api/v1/usage/dashboard/trend",
			query:        "start_date=2025-03-01&end_date=2025-03-23&granularity=day",
		},
		{
			name:         "dashboard trend",
			routePath:    "/dashboard/trend",
			upstreamPath: "/api/v1/usage/dashboard/trend",
			query:        "start_date=2025-03-01&end_date=2025-03-23&granularity=day",
		},
		{
			name:         "dashboard models",
			routePath:    "/dashboard/models",
			upstreamPath: "/api/v1/usage/dashboard/models",
			query:        "timeframe=30d",
		},
		{
			name:         "dashboard usage",
			routePath:    "/dashboard/usage",
			upstreamPath: "/api/v1/usage",
			query:        "page=2&per_page=50",
		},
		{
			name:         "subscription progress",
			routePath:    "/subscription",
			upstreamPath: "/api/v1/subscriptions/progress",
		},
		{
			name:         "dashboard account",
			routePath:    "/dashboard/account",
			upstreamPath: "/api/v1/user/profile",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			database := setupTestDB(t)

			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet {
					t.Fatalf("expected method %s, got %s", http.MethodGet, req.Method)
				}
				if req.URL.Path != tc.upstreamPath {
					t.Fatalf("expected upstream path %s, got %s", tc.upstreamPath, req.URL.Path)
				}
				expectedQuery, err := url.ParseQuery(tc.query)
				if err != nil {
					t.Fatalf("parse expected query: %v", err)
				}
				actualQuery, err := url.ParseQuery(req.URL.RawQuery)
				if err != nil {
					t.Fatalf("parse actual query: %v", err)
				}
				if !reflect.DeepEqual(actualQuery, expectedQuery) {
					t.Fatalf("expected query values %v, got %v", expectedQuery, actualQuery)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"source":"dashboard-upstream"}`))
			}))
			t.Cleanup(upstream.Close)

			proxyClient, err := proxy.NewClient(upstream.URL)
			if err != nil {
				t.Fatalf("create proxy client: %v", err)
			}

			m := http.NewServeMux()
			RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

			path := tc.routePath
			if tc.query != "" {
				path = path + "?" + tc.query
			}
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer token-123")
			req.Header.Set("X-User-Id", "999")
			rec := httptest.NewRecorder()
			m.ServeHTTP(rec, req)

			if rec.Code != http.StatusAccepted {
				t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
			}
			if rec.Body.String() != `{"source":"dashboard-upstream"}` {
				t.Fatalf("expected passthrough body %q, got %q", `{"source":"dashboard-upstream"}`, rec.Body.String())
			}
		})
	}
}

func TestDashboardPassthroughCopiesUpstreamStatusBodyAndHeaders(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="sub2api"`)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	m := http.NewServeMux()
	RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/account", nil)
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if rec.Body.String() != `{"error":"forbidden"}` {
		t.Fatalf("expected passthrough body %q, got %q", `{"error":"forbidden"}`, rec.Body.String())
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != `Bearer realm="sub2api"` {
		t.Fatalf("expected passthrough WWW-Authenticate, got %q", got)
	}
}

func TestDashboardModelsAndUsagePassthroughCopiesUpstreamStatusBodyAndHeaders(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		routePath string
	}{
		{name: "dashboard trend", routePath: "/dashboard/trend"},
		{name: "dashboard models", routePath: "/dashboard/models"},
		{name: "dashboard usage", routePath: "/dashboard/usage"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			database := setupTestDB(t)

			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="sub2api"`)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limited"}`))
			}))
			t.Cleanup(upstream.Close)

			proxyClient, err := proxy.NewClient(upstream.URL)
			if err != nil {
				t.Fatalf("create proxy client: %v", err)
			}

			m := http.NewServeMux()
			RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

			req := httptest.NewRequest(http.MethodGet, tc.routePath, nil)
			rec := httptest.NewRecorder()
			m.ServeHTTP(rec, req)

			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
			}
			if rec.Body.String() != `{"error":"rate limited"}` {
				t.Fatalf("expected passthrough body %q, got %q", `{"error":"rate limited"}`, rec.Body.String())
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != `Bearer realm="sub2api"` {
				t.Fatalf("expected passthrough WWW-Authenticate, got %q", got)
			}
		})
	}
}

func TestDashboardPassthroughReturnsBadGatewayWhenUpstreamUnavailable(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)

	proxyClient, err := proxy.NewClientWithHTTPClient("http://127.0.0.1:1", &http.Client{})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	m := http.NewServeMux()
	RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/home", nil)
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, rec.Code)
	}
	if rec.Body.String() == "" {
		t.Fatalf("expected non-empty error body")
	}
}
