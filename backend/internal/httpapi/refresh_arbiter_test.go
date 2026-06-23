package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"ai-api-portal/backend/internal/proxy"
)

// fakeAccessJWT builds an unsigned JWT carrying only `exp`. The arbiter reads
// exp from the payload without verifying the signature (sub2api is authoritative
// for real verification), so this is enough to drive the access-token-expiry
// cache path in tests.
func fakeAccessJWT(t *testing.T, expUnix int64) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{"exp": expUnix})
	return base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

type arbiterHarness struct {
	mux *http.ServeMux
	db  *sql.DB
}

func setupArbiterHarness(t *testing.T, handler http.HandlerFunc) *arbiterHarness {
	t.Helper()
	database := setupTestDB(t)
	upstream := httptest.NewServer(handler)
	t.Cleanup(upstream.Close)
	proxyClient, err := proxy.NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{ProxyClient: proxyClient})
	return &arbiterHarness{mux: mux, db: database}
}

func (h *arbiterHarness) seedVault(t *testing.T, userID int64, access, refresh string) {
	t.Helper()
	ctx := context.Background()
	if _, err := h.db.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, upstream_user_id, access_token, refresh_token) VALUES (?, NULL, ?, ?);`, userID, access, refresh); err != nil {
		t.Fatalf("seed vault: %v", err)
	}
}

func (h *arbiterHarness) postRefresh(t *testing.T, refreshToken string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader([]byte(`{"refresh_token":"`+refreshToken+`"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func (h *arbiterHarness) vault(t *testing.T, userID int64) (access, refresh, prev string, exists bool) {
	t.Helper()
	var (
		a    string
		r, p sql.NullString
	)
	err := h.db.QueryRowContext(context.Background(), `SELECT access_token, refresh_token, prev_refresh_token FROM als_sub2api_auth_tokens WHERE user_id = ?;`, userID).Scan(&a, &r, &p)
	if err == sql.ErrNoRows {
		return "", "", "", false
	}
	if err != nil {
		t.Fatalf("scan vault: %v", err)
	}
	return a, r.String, p.String, true
}

// TestRefreshArbiterDedupesAndRotates proves the multi-device-safe invariants:
// a real rotation calls sub2api once and stores current+prev; subsequent
// refreshes (whether the client holds current or the previous token) are served
// from cache without another upstream call, so sub2api never sees a reuse.
func TestRefreshArbiterDedupesAndRotates(t *testing.T) {
	t.Parallel()

	const rotatedAccess = "rotated-access"
	const rotatedRefresh = "R1"
	var calls int32

	h := setupArbiterHarness(t, func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/auth/refresh" {
			t.Errorf("unexpected upstream path: %s", req.URL.Path)
		}
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"` + rotatedAccess + `","refresh_token":"` + rotatedRefresh + `","expires_in":3600,"token_type":"Bearer"}}`))
	})

	userID, _ := createUserViaAPI(t, h.mux, "arbiter@example.com", "Arbiter User", "user", "")
	h.seedVault(t, userID, "A0", "R0")

	// 1) First refresh: access expiry unknown → rotate upstream once.
	if status, body := h.postRefresh(t, "R0"); status != http.StatusOK {
		t.Fatalf("first refresh: expected 200, got %d body=%s", status, body)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 upstream call after first refresh, got %d", got)
	}
	access, refresh, prev, exists := h.vault(t, userID)
	if !exists || access != rotatedAccess || refresh != rotatedRefresh || prev != "R0" {
		t.Fatalf("after rotation expected access=%q refresh=%q prev=%q exists=%v, got access=%q refresh=%q prev=%q exists=%v",
			rotatedAccess, rotatedRefresh, "R0", true, access, refresh, prev, exists)
	}

	// 2) Second refresh holding the CURRENT token → served from cache, no upstream call.
	if status, body := h.postRefresh(t, rotatedRefresh); status != http.StatusOK {
		t.Fatalf("second refresh: expected 200, got %d body=%s", status, body)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("dedup must not call upstream again; expected 1 call, got %d", got)
	}

	// 3) Third refresh holding the PREVIOUS token (grace window) → still served
	//    from cache and the caller converges onto the current refresh_token.
	if status, body := h.postRefresh(t, "R0"); status != http.StatusOK {
		t.Fatalf("grace refresh: expected 200, got %d body=%s", status, body)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("grace refresh must not call upstream; expected 1 call, got %d", got)
	}
}

// TestRefreshArbiterRejectsStaleToken ensures an unknown / too-stale refresh
// token is rejected with 401 and never forwarded upstream (forwarding it would
// risk tripping sub2api's refresh-token replay detection).
func TestRefreshArbiterRejectsStaleToken(t *testing.T) {
	t.Parallel()

	var calls int32
	h := setupArbiterHarness(t, func(w http.ResponseWriter, req *http.Request) {
		t.Errorf("stale-token refresh must not reach upstream: %s", req.URL.Path)
	})
	userID, _ := createUserViaAPI(t, h.mux, "stale@example.com", "Stale User", "user", "")
	h.seedVault(t, userID, "A0", "R0")

	if status, _ := h.postRefresh(t, "never-seen-token"); status != http.StatusUnauthorized {
		t.Fatalf("expected 401 for stale token, got %d", status)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("expected 0 upstream calls, got %d", got)
	}
}

// TestRefreshArbiterClearsVaultOnUpstreamRejection verifies that when sub2api
// rejects a rotation (family is dead), the vault is cleared so the user is
// forced to re-authenticate instead of the server repeatedly forwarding a
// doomed token.
func TestRefreshArbiterClearsVaultOnUpstreamRejection(t *testing.T) {
	t.Parallel()

	var calls int32
	h := setupArbiterHarness(t, func(w http.ResponseWriter, req *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":401,"message":"invalid refresh token"}`))
	})
	userID, _ := createUserViaAPI(t, h.mux, "doomed@example.com", "Doomed User", "user", "")
	h.seedVault(t, userID, "A0", "R0")

	if status, _ := h.postRefresh(t, "R0"); status != http.StatusUnauthorized {
		t.Fatalf("expected 401 from rejected rotation, got %d", status)
	}
	_, _, _, exists := h.vault(t, userID)
	if exists {
		t.Fatalf("vault should have been cleared after upstream rejection")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 upstream call (the failed rotation), got %d", got)
	}
}

// TestRefreshArbiterServesCachedFreshToken covers the path where the vault
// already holds a not-yet-expiring access token (populated by a prior login):
// a refresh is answered purely from cache with zero upstream calls.
func TestRefreshArbiterServesCachedFreshToken(t *testing.T) {
	t.Parallel()

	var calls int32
	h := setupArbiterHarness(t, func(w http.ResponseWriter, req *http.Request) {
		t.Errorf("fresh-cache refresh must not reach upstream: %s", req.URL.Path)
	})
	userID, _ := createUserViaAPI(t, h.mux, "fresh@example.com", "Fresh User", "user", "")
	// Seed a vault whose access_token is a JWT with a far-future exp; the login
	// capture path stores access_expires_at the same way.
	fresh := fakeAccessJWT(t, 4_102_444_800) // 2100-01-01
	ctx := context.Background()
	if _, err := h.db.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, upstream_user_id, access_token, refresh_token, access_expires_at) VALUES (?, NULL, ?, ?, ?);`, userID, fresh, "R0", "2099-01-01 00:00:00"); err != nil {
		t.Fatalf("seed fresh vault: %v", err)
	}

	status, body := h.postRefresh(t, "R0")
	if status != http.StatusOK {
		t.Fatalf("expected 200 from cached refresh, got %d body=%s", status, body)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("fresh cache must not call upstream, got %d", got)
	}
	if !strings.Contains(body, fresh) {
		t.Fatalf("cached response should contain the stored access token; body=%s", body)
	}
	if !strings.Contains(body, `"refresh_token":"R0"`) {
		t.Fatalf("cached response should return the current refresh token; body=%s", body)
	}
}
