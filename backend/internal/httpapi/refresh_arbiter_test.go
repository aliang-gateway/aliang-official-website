package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ai-api-portal/backend/internal/auth"
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestUpstreamRefreshDefinitelyInvalid(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, want: true},
		{name: "forbidden", status: http.StatusForbidden, want: true},
		{name: "bad request invalid token", status: http.StatusBadRequest, body: `{"reason":"REFRESH_TOKEN_INVALID"}`, want: true},
		{name: "bad request schema", status: http.StatusBadRequest, body: `{"message":"missing field"}`},
		{name: "rate limited", status: http.StatusTooManyRequests, body: `{"message":"rate limited"}`},
		{name: "upstream failure", status: http.StatusBadGateway},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := upstreamRefreshDefinitelyInvalid(tc.status, []byte(tc.body)); got != tc.want {
				t.Fatalf("upstreamRefreshDefinitelyInvalid() = %v, want %v", got, tc.want)
			}
		})
	}
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

func (h *arbiterHarness) mintSession(t *testing.T, userID int64) string {
	t.Helper()
	plaintext, tokenHash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("mint local session: %v", err)
	}
	if _, err := h.db.ExecContext(context.Background(), `INSERT INTO als_sessions(user_id, token_hash, expires_at) VALUES (?, ?, ?);`, userID, tokenHash, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("persist local session: %v", err)
	}
	return plaintext
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
// each device presents its own local session while the broker rotates one shared
// upstream family. Upstream tokens never appear in public responses.
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
	deviceA := h.mintSession(t, userID)
	deviceB := h.mintSession(t, userID)

	// 1) First refresh: access expiry unknown → rotate upstream once.
	if status, body := h.postRefresh(t, deviceA); status != http.StatusOK {
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

	// 2) Device B uses its independent local session and is served from cache.
	if status, body := h.postRefresh(t, deviceB); status != http.StatusOK {
		t.Fatalf("second refresh: expected 200, got %d body=%s", status, body)
	} else if !strings.Contains(body, deviceB) || strings.Contains(body, rotatedRefresh) || strings.Contains(body, rotatedAccess) {
		t.Fatalf("public refresh must return only device B local session; body=%s", body)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("dedup must not call upstream again; expected 1 call, got %d", got)
	}

	// 3) Device A remains valid after device B's refresh.
	if status, body := h.postRefresh(t, deviceA); status != http.StatusOK {
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
	session := h.mintSession(t, userID)

	if status, _ := h.postRefresh(t, session); status != http.StatusUnauthorized {
		t.Fatalf("expected 401 from rejected rotation, got %d", status)
	}
	_, _, _, exists := h.vault(t, userID)
	if exists {
		t.Fatalf("vault should have been cleared after upstream rejection")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 upstream call (the failed rotation), got %d", got)
	}
	if _, found, err := (&routes{db: h.db}).findLocalUserIDBySessionToken(context.Background(), session); err != nil || found {
		t.Fatalf("definitively invalid upstream family must revoke local sessions, found=%v err=%v", found, err)
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
	session := h.mintSession(t, userID)
	// Seed a vault whose access_token is a JWT with a far-future exp; the login
	// capture path stores access_expires_at the same way.
	fresh := fakeAccessJWT(t, 4_102_444_800) // 2100-01-01
	ctx := context.Background()
	if _, err := h.db.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, upstream_user_id, access_token, refresh_token, access_expires_at) VALUES (?, NULL, ?, ?, ?);`, userID, fresh, "R0", "2099-01-01 00:00:00"); err != nil {
		t.Fatalf("seed fresh vault: %v", err)
	}

	status, body := h.postRefresh(t, session)
	if status != http.StatusOK {
		t.Fatalf("expected 200 from cached refresh, got %d body=%s", status, body)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("fresh cache must not call upstream, got %d", got)
	}
	if strings.Contains(body, fresh) || strings.Contains(body, `"refresh_token":"R0"`) {
		t.Fatalf("cached response leaked upstream credentials; body=%s", body)
	}
	if !strings.Contains(body, session) {
		t.Fatalf("cached response should return the device local session; body=%s", body)
	}
}

func TestRefreshArbiterRetainsVaultOnTransientUpstreamFailure(t *testing.T) {
	t.Parallel()
	var calls int32
	h := setupArbiterHarness(t, func(w http.ResponseWriter, req *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":429,"message":"rate limited"}`))
	})
	userID, _ := createUserViaAPI(t, h.mux, "transient@example.com", "Transient User", "user", "")
	h.seedVault(t, userID, "A0", "R0")
	session := h.mintSession(t, userID)

	if status, _ := h.postRefresh(t, session); status != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", status)
	}
	_, refresh, _, exists := h.vault(t, userID)
	if !exists || refresh != "R0" {
		t.Fatalf("transient failure must retain vault, exists=%v refresh=%q", exists, refresh)
	}
	var state string
	if err := h.db.QueryRow(`SELECT rotation_state FROM als_sub2api_auth_tokens WHERE user_id = ?`, userID).Scan(&state); err != nil {
		t.Fatalf("query rotation state: %v", err)
	}
	if state != "stable" {
		t.Fatalf("rotation state=%q, want stable", state)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("upstream calls=%d, want 1", got)
	}
}

func TestRefreshArbiterNeverReplaysUncertainRotation(t *testing.T) {
	t.Parallel()
	var calls int32
	h := setupArbiterHarness(t, func(w http.ResponseWriter, req *http.Request) {
		atomic.AddInt32(&calls, 1)
		t.Errorf("uncertain rotation must not reach upstream: %s", req.URL.Path)
	})
	userID, _ := createUserViaAPI(t, h.mux, "uncertain@example.com", "Uncertain User", "user", "")
	h.seedVault(t, userID, "A0", "R0")
	session := h.mintSession(t, userID)
	if _, err := h.db.Exec(`UPDATE als_sub2api_auth_tokens SET rotation_state = 'uncertain' WHERE user_id = ?`, userID); err != nil {
		t.Fatalf("mark uncertain: %v", err)
	}

	if status, _ := h.postRefresh(t, session); status != http.StatusUnauthorized {
		t.Fatalf("expected 401 for uncertain rotation, got %d", status)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("upstream calls=%d, want 0", got)
	}
	if _, found, err := (&routes{db: h.db}).findLocalUserIDBySessionToken(context.Background(), session); err != nil || found {
		t.Fatalf("uncertain upstream family must revoke local sessions, found=%v err=%v", found, err)
	}
}

func TestRefreshArbiterMarksTransportFailureUncertain(t *testing.T) {
	t.Parallel()
	database := setupTestDB(t)
	proxyClient, err := proxy.NewClientWithHTTPClient("http://sub2api.invalid", &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("ambiguous connection reset")
	})})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{ProxyClient: proxyClient})
	h := &arbiterHarness{mux: mux, db: database}
	userID, _ := createUserViaAPI(t, mux, "transport@example.com", "Transport User", "user", "")
	h.seedVault(t, userID, "A0", "R0")
	session := h.mintSession(t, userID)

	if status, _ := h.postRefresh(t, session); status != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for ambiguous transport failure, got %d", status)
	}
	var state string
	if err := database.QueryRow(`SELECT rotation_state FROM als_sub2api_auth_tokens WHERE user_id = ?`, userID).Scan(&state); err != nil {
		t.Fatalf("query rotation state: %v", err)
	}
	if state != "uncertain" {
		t.Fatalf("rotation state=%q, want uncertain", state)
	}
	if _, found, err := (&routes{db: database}).findLocalUserIDBySessionToken(context.Background(), session); err != nil || found {
		t.Fatalf("ambiguous rotation must revoke local sessions, found=%v err=%v", found, err)
	}
}

// decodeRefreshAnchorPayload extracts the anchor-relevant fields from a refresh
// arbiter response body.
func decodeRefreshAnchorPayload(t *testing.T, body string) (upstreamIn, upstreamAt, expiresIn int64) {
	t.Helper()
	var payload struct {
		Data struct {
			ExpiresIn         int64 `json:"expires_in"`
			UpstreamExpiresIn int64 `json:"upstream_expires_in"`
			UpstreamExpiresAt int64 `json:"upstream_expires_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode refresh anchor payload: %v (body=%s)", err, body)
	}
	return payload.Data.UpstreamExpiresIn, payload.Data.UpstreamExpiresAt, payload.Data.ExpiresIn
}

func assertCloseSeconds(t *testing.T, label string, got, want, tolerance int64) {
	t.Helper()
	if diff := got - want; diff > tolerance || diff < -tolerance {
		t.Fatalf("%s = %d, want ≈%d (±%ds)", label, got, want, tolerance)
	}
}

// TestRefreshArbiterResponseCarriesUpstreamAnchor proves the refresh response
// exposes the upstream access token's real expiry (upstream_expires_in /
// upstream_expires_at) alongside the unchanged rolling-local expires_in, so
// clients can anchor their credential countdown on the constraint that actually
// forces a re-authentication.
func TestRefreshArbiterResponseCarriesUpstreamAnchor(t *testing.T) {
	t.Parallel()

	// Scenario 1 — cached-fresh vault with a known access_expires_at: the anchor
	// mirrors it exactly, with zero upstream calls.
	h := setupArbiterHarness(t, func(w http.ResponseWriter, req *http.Request) {
		t.Errorf("cached-fresh anchor refresh must not reach upstream: %s", req.URL.Path)
	})
	userID, _ := createUserViaAPI(t, h.mux, "anchor-fresh@example.com", "Anchor Fresh", "user", "")
	session := h.mintSession(t, userID)
	anchorTTL := 7*time.Hour + 30*time.Minute
	anchorAt := time.Now().UTC().Add(anchorTTL)
	if _, err := h.db.ExecContext(context.Background(), `
		INSERT INTO als_sub2api_auth_tokens(user_id, upstream_user_id, access_token, refresh_token, access_expires_at)
		VALUES (?, NULL, ?, ?, ?);
	`, userID, fakeAccessJWT(t, anchorAt.Unix()), "R0", anchorAt); err != nil {
		t.Fatalf("seed anchored vault: %v", err)
	}

	status, body := h.postRefresh(t, session)
	if status != http.StatusOK {
		t.Fatalf("expected 200 from anchored cached refresh, got %d body=%s", status, body)
	}
	upstreamIn, upstreamAt, expiresIn := decodeRefreshAnchorPayload(t, body)
	assertCloseSeconds(t, "data.upstream_expires_in", upstreamIn, int64(anchorTTL.Seconds()), 60)
	assertCloseSeconds(t, "data.upstream_expires_at", upstreamAt, anchorAt.Unix(), 60)
	if expiresIn <= 0 {
		t.Fatalf("local expires_in must stay a positive rolling value, got %d (body=%s)", expiresIn, body)
	}

	// Scenario 2 — vault with NULL access_expires_at plus an opaque rotated
	// access token: the persisted expiry comes from the defaultAccessTTL
	// fallback, and the response anchor mirrors that ~50-minute window.
	h2 := setupArbiterHarness(t, func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/auth/refresh" {
			t.Errorf("unexpected upstream path: %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"rotated-opaque-access","refresh_token":"R1","expires_in":3600,"token_type":"Bearer"}}`))
	})
	userID2, _ := createUserViaAPI(t, h2.mux, "anchor-fallback@example.com", "Anchor Fallback", "user", "")
	session2 := h2.mintSession(t, userID2)
	h2.seedVault(t, userID2, "A0", "R0") // NULL access_expires_at forces one rotation

	status2, body2 := h2.postRefresh(t, session2)
	if status2 != http.StatusOK {
		t.Fatalf("expected 200 from fallback anchored refresh, got %d body=%s", status2, body2)
	}
	upstreamIn2, _, expiresIn2 := decodeRefreshAnchorPayload(t, body2)
	assertCloseSeconds(t, "data.upstream_expires_in (defaultAccessTTL fallback)", upstreamIn2, int64(defaultAccessTTL.Seconds()), 60)
	if expiresIn2 <= 0 {
		t.Fatalf("local expires_in must stay a positive rolling value, got %d (body=%s)", expiresIn2, body2)
	}

	// Scenario 3 — vault whose access_expires_at is already in the past (and
	// whose rotated upstream access token is likewise an expired JWT): the
	// anchor must floor at 1 instead of reporting a negative countdown.
	expiredAt := time.Now().UTC().Add(-time.Hour)
	expiredAccess := fakeAccessJWT(t, expiredAt.Unix())
	h3 := setupArbiterHarness(t, func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/auth/refresh" {
			t.Errorf("unexpected upstream path: %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"` + expiredAccess + `","refresh_token":"R1","expires_in":3600,"token_type":"Bearer"}}`))
	})
	userID3, _ := createUserViaAPI(t, h3.mux, "anchor-clamp@example.com", "Anchor Clamp", "user", "")
	session3 := h3.mintSession(t, userID3)
	if _, err := h3.db.ExecContext(context.Background(), `
		INSERT INTO als_sub2api_auth_tokens(user_id, upstream_user_id, access_token, refresh_token, access_expires_at)
		VALUES (?, NULL, ?, ?, ?);
	`, userID3, expiredAccess, "R0", expiredAt); err != nil {
		t.Fatalf("seed expired vault: %v", err)
	}

	status3, body3 := h3.postRefresh(t, session3)
	if status3 != http.StatusOK {
		t.Fatalf("expected 200 from clamped anchored refresh, got %d body=%s", status3, body3)
	}
	upstreamIn3, _, expiresIn3 := decodeRefreshAnchorPayload(t, body3)
	if upstreamIn3 != 1 {
		t.Fatalf("data.upstream_expires_in = %d, want 1 (floor clamp) (body=%s)", upstreamIn3, body3)
	}
	if expiresIn3 <= 0 {
		t.Fatalf("local expires_in must stay a positive rolling value, got %d (body=%s)", expiresIn3, body3)
	}
}
