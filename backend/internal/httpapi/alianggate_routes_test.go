package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ai-api-portal/backend/internal/proxy"
)

// alianggateClientRoutes lists paths the alianggate desktop client calls on core.api_server.
// Each must be registered on the portal backend (not return mux 404).
var alianggateClientRoutes = []struct {
	method string
	path   string
}{
	{"POST", "/api/v1/auth/login"},
	{"POST", "/api/v1/auth/refresh"},
	{"POST", "/api/v1/auth/logout"},
	{"GET", "/api/v1/auth/me"},
	{"GET", "/api/v1/user/profile"},
	{"PUT", "/api/v1/user"},
	{"GET", "/api/v1/subscriptions/summary"},
	{"GET", "/api/v1/subscriptions/progress"},
	{"GET", "/api/v1/groups/available"},
	{"GET", "/api/v1/keys"},
	{"POST", "/api/v1/redeem"},
	{"GET", "/api/v1/usage/dashboard/stats"},
	{"GET", "/api/v1/usage/dashboard/trend"},
	{"GET", "/api/v1/usage/dashboard/models"},
	{"GET", "/api/v1/usage"},
	{"GET", "/api/v1/usage/stats"},
	{"GET", "/api/v1/admin/ops/dashboard/snapshot-v2"},
	{"GET", "/api/public/downloads/check"},
}

func TestAlianggateClientRoutesAreRegistered(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	m := http.NewServeMux()
	RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

	for _, tc := range alianggateClientRoutes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer test-token")
			rec := httptest.NewRecorder()
			m.ServeHTTP(rec, req)

			if rec.Code == http.StatusNotFound {
				t.Fatalf("route not registered: %s %s returned 404", tc.method, tc.path)
			}
		})
	}
}

func TestAlianggateUserProfileProxiesUpstream(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)

	var sawProfile bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/v1/user/profile" {
			sawProfile = true
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"id":1,"email":"u@example.com","username":"u","role":"user","balance":0,"concurrency":1,"status":"active","allowed_groups":[],"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"},"message":"ok"}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	m := http.NewServeMux()
	RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/profile", nil)
	req.Header.Set("Authorization", "Bearer upstream-access-token")
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Fatalf("expected profile route to be registered, got 404")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if !sawProfile {
		t.Fatalf("expected upstream /api/v1/user/profile to be called")
	}
}
