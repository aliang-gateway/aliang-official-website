package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"ai-api-portal/backend/internal/proxy"
)

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
