package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ai-api-portal/backend/internal/auth"
	"ai-api-portal/backend/internal/user"
)

// doJSON issues a JSON request against the test mux and returns the status code
// plus the decoded JSON body (which may be nil for empty/non-JSON responses).
func doJSON(t *testing.T, mux *http.ServeMux, method, path string, body any, bearer string) (int, map[string]any) {
	t.Helper()

	var rdr *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		rdr = bytes.NewReader(raw)
	} else {
		rdr = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		setBearerAuth(req, bearer)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var payload map[string]any
	if rec.Body.Len() > 0 && strings.Contains(rec.Header().Get("Content-Type"), "json") {
		_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	}
	return rec.Code, payload
}

func TestScanLoginFullFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)

	// Seed an App user and mint its session token (acts as the App credential).
	appUserID, err := database.ExecContext(ctx, `
		INSERT INTO als_users(email, name, role) VALUES (?, ?, ?);
	`, "app@example.com", "App User", "user")
	if err != nil {
		t.Fatalf("seed app user: %v", err)
	}
	id, err := appUserID.LastInsertId()
	if err != nil {
		t.Fatalf("app user LastInsertId: %v", err)
	}
	appUserIDVal := id

	userSvc := user.NewService(database)
	appToken, _, err := userSvc.MintSessionForUser(ctx, appUserIDVal)
	if err != nil {
		t.Fatalf("mint app session: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{SQLDialect: "sqlite"})

	// 1. Init (no auth) → 200, capture device_code + scan_code.
	status, payload := doJSON(t, mux, http.MethodPost, "/auth/scan/init", map[string]string{}, "")
	if status != http.StatusOK {
		t.Fatalf("init status = %d, body=%v", status, payload)
	}
	deviceCode, _ := payload["device_code"].(string)
	scanCode, _ := payload["scan_code"].(string)
	if deviceCode == "" || scanCode == "" {
		t.Fatalf("init missing codes: device=%q scan=%q", deviceCode, scanCode)
	}

	// 2. Status → pending.
	status, payload = doJSON(t, mux, http.MethodGet, "/auth/scan/status?device_code="+deviceCode, nil, "")
	if status != http.StatusOK {
		t.Fatalf("status#1 = %d, body=%v", status, payload)
	}
	if got, _ := payload["status"].(string); got != "pending" {
		t.Fatalf("status#1 status = %q, want pending", got)
	}

	// 3. Scan with no auth → 401.
	status, _ = doJSON(t, mux, http.MethodPost, "/auth/scan/scan", map[string]string{"code": scanCode}, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("scan no-auth status = %d, want 401", status)
	}

	// 4. Scan with App bearer → 200.
	status, _ = doJSON(t, mux, http.MethodPost, "/auth/scan/scan", map[string]string{"code": scanCode}, appToken)
	if status != http.StatusOK {
		t.Fatalf("scan status = %d, want 200", status)
	}

	// 5. Status → scanned.
	status, payload = doJSON(t, mux, http.MethodGet, "/auth/scan/status?device_code="+deviceCode, nil, "")
	if status != http.StatusOK {
		t.Fatalf("status#2 = %d, body=%v", status, payload)
	}
	if got, _ := payload["status"].(string); got != "scanned" {
		t.Fatalf("status#2 status = %q, want scanned", got)
	}

	// 6. Confirm with App bearer → 200.
	status, _ = doJSON(t, mux, http.MethodPost, "/auth/scan/confirm", map[string]string{"code": scanCode}, appToken)
	if status != http.StatusOK {
		t.Fatalf("confirm status = %d, want 200", status)
	}

	// 7. Status → authorized, session_token present (st_ prefix), user.id matches.
	status, payload = doJSON(t, mux, http.MethodGet, "/auth/scan/status?device_code="+deviceCode, nil, "")
	if status != http.StatusOK {
		t.Fatalf("status#3 = %d, body=%v", status, payload)
	}
	if got, _ := payload["status"].(string); got != "authorized" {
		t.Fatalf("status#3 status = %q, want authorized", got)
	}
	pcToken, _ := payload["session_token"].(string)
	if !strings.HasPrefix(pcToken, "st_") {
		t.Fatalf("session_token = %q, want st_ prefix", pcToken)
	}
	userObj, _ := payload["user"].(map[string]any)
	if userObj == nil {
		t.Fatalf("authorized status missing user object: %v", payload)
	}
	if got := int64(userObj["id"].(float64)); got != appUserIDVal {
		t.Fatalf("user.id = %d, want %d", got, appUserIDVal)
	}

	// 8. Validate the minted PC token passes auth.RequireUser against an
	//    authenticated local route. /auth/me is an upstream passthrough
	//    (would 500 without a proxyClient), so we use the local /user/me
	//    handler which resolves the user from context after the middleware
	//    validates the session hash against als_sessions.
	status, payload = doJSON(t, mux, http.MethodGet, "/user/me", nil, pcToken)
	if status != http.StatusOK {
		t.Fatalf("pc-token /user/me status = %d, want 200, body=%v", status, payload)
	}
	if got := int64(payload["id"].(float64)); got != appUserIDVal {
		t.Fatalf("pc-token user id = %d, want %d", got, appUserIDVal)
	}

	// Belt-and-suspenders: confirm als_sessions holds a row for the PC token hash.
	tokenHash := auth.HashSessionToken(pcToken)
	var count int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM als_sessions WHERE token_hash = ? AND revoked_at IS NULL;
	`, tokenHash).Scan(&count); err != nil {
		t.Fatalf("query als_sessions: %v", err)
	}
	if count != 1 {
		t.Fatalf("als_sessions count for pc token = %d, want 1", count)
	}
}

// TestScanLoginWithAccessToken verifies the App (the phone app) can authorize a
// desktop scan-login using the sub2api access_token it already holds — without
// an st_ session. The phone never has an st_; this is its credential.
func TestScanLoginWithAccessToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)

	// Seed a user and store their sub2api access_token (what the phone carries).
	res, err := database.ExecContext(ctx, `
		INSERT INTO als_users(email, name, role) VALUES (?, ?, ?);
	`, "phone@example.com", "Phone App", "user")
	if err != nil {
		t.Fatalf("seed phone user: %v", err)
	}
	phoneUserID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("phone user LastInsertId: %v", err)
	}
	const phoneAccessToken = "phone_sub2api_access_token_xyz"
	if _, err := database.ExecContext(ctx, `
		INSERT INTO als_sub2api_auth_tokens(user_id, access_token) VALUES (?, ?);
	`, phoneUserID, phoneAccessToken); err != nil {
		t.Fatalf("seed sub2api access token: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{SQLDialect: "sqlite"})

	// 1. Init → scan_code.
	_, payload := doJSON(t, mux, http.MethodPost, "/auth/scan/init", map[string]string{}, "")
	deviceCode, _ := payload["device_code"].(string)
	scanCode, _ := payload["scan_code"].(string)
	if deviceCode == "" || scanCode == "" {
		t.Fatalf("init missing codes: %v", payload)
	}

	// 2. Scan with the access_token bearer → 200, → scanned.
	status, _ := doJSON(t, mux, http.MethodPost, "/auth/scan/scan", map[string]string{"code": scanCode}, phoneAccessToken)
	if status != http.StatusOK {
		t.Fatalf("scan with access_token status = %d, want 200", status)
	}
	_, payload = doJSON(t, mux, http.MethodGet, "/auth/scan/status?device_code="+deviceCode, nil, "")
	if got, _ := payload["status"].(string); got != "scanned" {
		t.Fatalf("status after access_token scan = %q, want scanned", got)
	}

	// 3. Confirm with the access_token bearer → 200, → authorized; user is the
	//    phone user (resolved from the stored access_token).
	status, payload = doJSON(t, mux, http.MethodPost, "/auth/scan/confirm", map[string]string{"code": scanCode}, phoneAccessToken)
	if status != http.StatusOK {
		t.Fatalf("confirm with access_token status = %d, want 200", status)
	}
	_, payload = doJSON(t, mux, http.MethodGet, "/auth/scan/status?device_code="+deviceCode, nil, "")
	if got, _ := payload["status"].(string); got != "authorized" {
		t.Fatalf("status after access_token confirm = %q, want authorized", got)
	}
	userObj, _ := payload["user"].(map[string]any)
	if userObj == nil {
		t.Fatalf("authorized status missing user: %v", payload)
	}
	if got := int64(userObj["id"].(float64)); got != phoneUserID {
		t.Fatalf("authorized user.id = %d, want %d (phone user)", got, phoneUserID)
	}

	// 4. Regression: an st_ session token still authenticates (website path).
	userSvc := user.NewService(database)
	stToken, _, err := userSvc.MintSessionForUser(ctx, phoneUserID)
	if err != nil {
		t.Fatalf("mint st_ session: %v", err)
	}
	_, payload = doJSON(t, mux, http.MethodPost, "/auth/scan/init", map[string]string{}, "")
	scanCode2, _ := payload["scan_code"].(string)
	status, _ = doJSON(t, mux, http.MethodPost, "/auth/scan/scan", map[string]string{"code": scanCode2}, stToken)
	if status != http.StatusOK {
		t.Fatalf("scan with st_ status = %d, want 200 (regression)", status)
	}

	// 5. Unknown token → 401.
	status, _ = doJSON(t, mux, http.MethodPost, "/auth/scan/scan", map[string]string{"code": scanCode2}, "not_a_real_token")
	if status != http.StatusUnauthorized {
		t.Fatalf("scan with bogus token status = %d, want 401", status)
	}
}
