package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"ai-api-portal/backend/internal/auth"
	"ai-api-portal/backend/internal/proxy"
)

const (
	accountDeleteTestEmail      = "delete-me@example.com"
	accountDeleteTestPassword   = "correct-horse"
	accountDeleteTestUpstreamID = 4242
)

// sub2apiDeleteStub plays the upstream for the account deletion flow: the
// password re-auth login and the admin user delete both hit the same server,
// mirroring how the proxy client fronts one sub2api base URL.
type sub2apiDeleteStub struct {
	server *httptest.Server

	mu           sync.Mutex
	deleteStatus int      // status returned for DELETE /api/v1/admin/users/{id}
	adminDeletes []string // recorded delete paths
}

func newSub2apiDeleteStub(t *testing.T, deleteStatus int) *sub2apiDeleteStub {
	t.Helper()

	stub := &sub2apiDeleteStub{deleteStatus: deleteStatus}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/api/v1/auth/login":
			var payload map[string]string
			_ = json.NewDecoder(req.Body).Decode(&payload)
			if payload["email"] != accountDeleteTestEmail || payload["password"] != accountDeleteTestPassword {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"message":"invalid email or password"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fmt.Sprintf(`{"access_token":"reauth-at","refresh_token":"reauth-rt","user":{"id":%d,"email":%q}}`, accountDeleteTestUpstreamID, accountDeleteTestEmail)))
		case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/api/v1/admin/users/"):
			stub.mu.Lock()
			stub.adminDeletes = append(stub.adminDeletes, req.URL.Path)
			stub.mu.Unlock()
			if req.Header.Get("x-api-key") == "" {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"message":"missing admin key"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(stub.deleteStatus)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected upstream call: %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *sub2apiDeleteStub) deletePaths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.adminDeletes...)
}

// phoneServerDeleteStub plays the companion phone platform server's admin API
// (GET /api/admin/users + DELETE /api/admin/users/{id}).
type phoneServerDeleteStub struct {
	server *httptest.Server

	mu      sync.Mutex
	lists   int
	deletes []string
}

func newPhoneServerDeleteStub(t *testing.T) *phoneServerDeleteStub {
	t.Helper()

	stub := &phoneServerDeleteStub{}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("x-admin-token") == "" || req.Header.Get("x-admin-console") != "1" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"admin_only"}`))
			return
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/api/admin/users":
			stub.mu.Lock()
			stub.lists++
			stub.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fmt.Sprintf(`{"users":[{"id":"user_phone1","username":"deleteme","email":%q,"role":"user"}]}`, accountDeleteTestEmail)))
		case req.Method == http.MethodDelete && req.URL.Path == "/api/admin/users/user_phone1":
			stub.mu.Lock()
			stub.deletes = append(stub.deletes, req.URL.RequestURI())
			stub.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"deleted","user_id":"user_phone1"}`))
		default:
			t.Fatalf("unexpected phone server call: %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *phoneServerDeleteStub) deletePaths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.deletes...)
}

// seedDeletableAccount mints a local user with a sub2api vault row and one
// live session, returning the local user id and the plaintext session token.
func seedDeletableAccount(t *testing.T, ctx context.Context, database *sql.DB, withVaultUpstreamID bool) (int64, string) {
	t.Helper()

	userID := createUser(t, ctx, database, accountDeleteTestEmail, "Delete Me", "user")

	var vaultInsert string
	if withVaultUpstreamID {
		vaultInsert = fmt.Sprintf(`INSERT INTO als_sub2api_auth_tokens(user_id, access_token, refresh_token, upstream_user_id) VALUES (%d, 'vault-access', 'vault-refresh', %d);`, userID, accountDeleteTestUpstreamID)
	} else {
		vaultInsert = fmt.Sprintf(`INSERT INTO als_sub2api_auth_tokens(user_id, access_token, refresh_token) VALUES (%d, 'vault-access', 'vault-refresh');`, userID)
	}
	if _, err := database.ExecContext(ctx, vaultInsert); err != nil {
		t.Fatalf("seed als_sub2api_auth_tokens row: %v", err)
	}

	plaintext, tokenHash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("mint local session: %v", err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sessions(user_id, token_hash, expires_at) VALUES (?, ?, ?);`, userID, tokenHash, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("seed local session: %v", err)
	}

	return userID, plaintext
}

func countAccountDeleteRows(t *testing.T, ctx context.Context, database *sql.DB, query string, args ...any) int64 {
	t.Helper()

	var count int64
	if err := database.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		t.Fatalf("count rows (%s): %v", query, err)
	}
	return count
}

func TestHandleAuthAccountDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		authHeader       string
		requestBody      string
		upstreamDelete   int // status the stub returns for the admin delete
		configurePhone   bool
		wantStatus       int
		wantBodyStatus   string // expected "status" field, "" for error bodies
		wantAdminDeletes int
		wantPhoneDeletes int
		wantLocalRows    bool // local user/session/vault rows still present afterwards
	}{
		{
			name:             "missing password is rejected before any upstream call",
			requestBody:      `{"password":""}`,
			wantStatus:       http.StatusBadRequest,
			wantAdminDeletes: 0,
			wantPhoneDeletes: 0,
			wantLocalRows:    true,
		},
		{
			name:             "unauthenticated request is rejected",
			requestBody:      `{"password":"` + accountDeleteTestPassword + `"}`,
			wantStatus:       http.StatusUnauthorized,
			wantAdminDeletes: 0,
			wantPhoneDeletes: 0,
			wantLocalRows:    true,
		},
		{
			name:             "wrong password fails re-auth and keeps every local row",
			requestBody:      `{"password":"wrong-password"}`,
			wantStatus:       http.StatusUnauthorized,
			wantAdminDeletes: 0,
			wantPhoneDeletes: 0,
			wantLocalRows:    true,
		},
		{
			name:             "happy path deletes upstream then purges local rows",
			requestBody:      `{"password":"` + accountDeleteTestPassword + `"}`,
			upstreamDelete:   http.StatusOK,
			configurePhone:   true,
			wantStatus:       http.StatusOK,
			wantBodyStatus:   "deleted",
			wantAdminDeletes: 1,
			wantPhoneDeletes: 1,
		},
		{
			name:             "sub2api 404 counts as already deleted",
			requestBody:      `{"password":"` + accountDeleteTestPassword + `"}`,
			upstreamDelete:   http.StatusNotFound,
			wantStatus:       http.StatusOK,
			wantBodyStatus:   "deleted",
			wantAdminDeletes: 1,
			wantPhoneDeletes: 0,
		},
		{
			name:             "sub2api 500 fails the deletion and keeps every local row",
			requestBody:      `{"password":"` + accountDeleteTestPassword + `"}`,
			upstreamDelete:   http.StatusInternalServerError,
			wantStatus:       http.StatusBadGateway,
			wantAdminDeletes: 1,
			wantPhoneDeletes: 0,
			wantLocalRows:    true,
		},
		{
			name:             "phone server purge is skipped when not configured",
			requestBody:      `{"password":"` + accountDeleteTestPassword + `"}`,
			upstreamDelete:   http.StatusOK,
			configurePhone:   false,
			wantStatus:       http.StatusOK,
			wantBodyStatus:   "deleted",
			wantAdminDeletes: 1,
			wantPhoneDeletes: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			database := setupTestDB(t)
			userID, sessionToken := seedDeletableAccount(t, ctx, database, true)

			upstreamStub := newSub2apiDeleteStub(t, tt.upstreamDelete)
			phoneStub := newPhoneServerDeleteStub(t)

			proxyClient, err := proxy.NewClientWithOptions(upstreamStub.server.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "test-admin-key"})
			if err != nil {
				t.Fatalf("create proxy client: %v", err)
			}

			opts := RoutesOptions{ProxyClient: proxyClient}
			if tt.configurePhone {
				opts.PhoneServerAdminURL = phoneStub.server.URL
				opts.PhoneServerAdminToken = "phone-admin-token"
			}
			m := http.NewServeMux()
			RegisterRoutesWithOptions(m, database, opts)

			req := httptest.NewRequest(http.MethodDelete, "/api/auth/account", bytes.NewReader([]byte(tt.requestBody)))
			if tt.authHeader == "" && tt.name != "unauthenticated request is rejected" {
				setBearerAuth(req, sessionToken)
			} else if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			m.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			var payload map[string]any
			if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
				t.Fatalf("decode response body: %v", err)
			}
			if tt.wantBodyStatus == "" {
				if _, hasError := payload["error"]; !hasError {
					t.Fatalf("expected error payload, got %v", payload)
				}
			} else if payload["status"] != tt.wantBodyStatus {
				t.Fatalf("expected status %q, got %v", tt.wantBodyStatus, payload["status"])
			}

			if got := len(upstreamStub.deletePaths()); got != tt.wantAdminDeletes {
				t.Fatalf("upstream admin deletes = %d (%v), want %d", got, upstreamStub.deletePaths(), tt.wantAdminDeletes)
			}
			if got := len(phoneStub.deletePaths()); got != tt.wantPhoneDeletes {
				t.Fatalf("phone server deletes = %d, want %d", got, tt.wantPhoneDeletes)
			}
			// The mirror purge must request the full data wipe (purgeData=true),
			// not just the account row — self-service deletion removes personal
			// data too (App Store 5.1.1(v)).
			for _, uri := range phoneStub.deletePaths() {
				if uri != "/api/admin/users/user_phone1?purgeData=true" {
					t.Fatalf("phone server delete URI = %q, want purgeData=true query", uri)
				}
			}

			wantRowCount := int64(0)
			if tt.wantLocalRows {
				wantRowCount = 1
			}
			if got := countAccountDeleteRows(t, ctx, database, `SELECT COUNT(*) FROM als_users WHERE id = ?;`, userID); got != wantRowCount {
				t.Fatalf("als_users rows = %d, want %d", got, wantRowCount)
			}
			if got := countAccountDeleteRows(t, ctx, database, `SELECT COUNT(*) FROM als_sessions WHERE user_id = ?;`, userID); got != wantRowCount {
				t.Fatalf("als_sessions rows = %d, want %d", got, wantRowCount)
			}
			if got := countAccountDeleteRows(t, ctx, database, `SELECT COUNT(*) FROM als_sub2api_auth_tokens WHERE user_id = ?;`, userID); got != wantRowCount {
				t.Fatalf("als_sub2api_auth_tokens rows = %d, want %d", got, wantRowCount)
			}
		})
	}
}

// TestHandleAuthAccountDeleteResolvesUpstreamIDFromLogin covers the fallback
// path where the vault has no upstream_user_id yet: the id embedded in the
// re-auth login response must be used for the admin delete.
func TestHandleAuthAccountDeleteResolvesUpstreamIDFromLogin(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	userID, sessionToken := seedDeletableAccount(t, ctx, database, false)

	upstreamStub := newSub2apiDeleteStub(t, http.StatusOK)
	proxyClient, err := proxy.NewClientWithOptions(upstreamStub.server.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "test-admin-key"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	m := http.NewServeMux()
	RegisterRoutesWithOptions(m, database, RoutesOptions{ProxyClient: proxyClient})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/account", bytes.NewReader([]byte(`{"password":"`+accountDeleteTestPassword+`"}`)))
	setBearerAuth(req, sessionToken)
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	deletes := upstreamStub.deletePaths()
	if len(deletes) != 1 || deletes[0] != fmt.Sprintf("/api/v1/admin/users/%d", accountDeleteTestUpstreamID) {
		t.Fatalf("expected one delete of /api/v1/admin/users/%d, got %v", accountDeleteTestUpstreamID, deletes)
	}
	if got := countAccountDeleteRows(t, ctx, database, `SELECT COUNT(*) FROM als_users WHERE id = ?;`, userID); got != 0 {
		t.Fatalf("expected local user row purged, got %d rows", got)
	}
}
