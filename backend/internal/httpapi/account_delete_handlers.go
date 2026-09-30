package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/proxy"
)

type deleteAccountRequest struct {
	Password string `json:"password"`
}

// accountDeleteUpstreamTimeout bounds every outbound call issued by the account
// deletion flow (upstream re-auth, sub2api admin delete, phone-server mirror
// purge) so a hung dependency cannot pin the request indefinitely.
const accountDeleteUpstreamTimeout = 10 * time.Second

// phoneServerHTTPClient serves the best-effort phone-server mirror purge; the
// per-request deadline comes from the purge context.
var phoneServerHTTPClient = &http.Client{Timeout: accountDeleteUpstreamTimeout}

// handleAuthAccountDelete implements self-service account deletion:
//
//  1. resolve the caller from the local session (same path as /auth/me),
//  2. re-verify the password against the same upstream the login passthrough
//     uses (Apple-sanctioned re-authentication),
//  3. delete the sub2api account via the admin API (authoritative, must
//     succeed — 404 counts as already deleted),
//  4. purge the local user row, sessions and sub2api token vault in one
//     transaction,
//  5. best-effort mirror the deletion on the companion phone platform server.
//
// A failed upstream delete leaves every local row untouched.
func (r *routes) handleAuthAccountDelete(w http.ResponseWriter, req *http.Request) {
	if r.proxyClient == nil {
		writeError(w, http.StatusInternalServerError, "auth proxy is not configured")
		return
	}

	var payload deleteAccountRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if strings.TrimSpace(payload.Password) == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}

	// Same identity path as handleAuthMePassthrough: bearer session token →
	// local user. Unlike /auth/me there is no upstream fall-through; a
	// destructive action requires a live local session.
	profile, handled, err := r.resolveLocalAuthMeProfile(req.Context(), req.Header.Get("Authorization"))
	if err != nil {
		slog.Error("resolveLocalAuthMeProfile error", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to resolve local session")
		return
	}
	if !handled {
		writeError(w, http.StatusUnauthorized, "local session is no longer valid")
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), accountDeleteUpstreamTimeout)
	defer cancel()

	// Re-authentication: replay the login credential check with the account
	// email + provided password. The freshly minted upstream tokens are
	// deliberately not captured — the account is about to disappear.
	reauthBody, reauthStatus, err := r.loginSub2APIUser(ctx, profile.Email, payload.Password)
	if err != nil {
		slog.Warn("account delete upstream re-auth failed", "user_id", profile.ID, "error", err)
		writeError(w, http.StatusBadGateway, "failed to verify credentials")
		return
	}
	if reauthStatus < 200 || reauthStatus >= 300 {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	upstreamUserID, found, err := r.sub2api.UpstreamUserID(ctx, profile.ID)
	if err != nil {
		slog.Error("account delete failed to read sub2api vault", "user_id", profile.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to resolve upstream account")
		return
	}
	if !found {
		// Fall back to the id embedded in the re-auth login response (same
		// extraction the login passthrough uses to seed the vault).
		if _, _, _, fromLogin, ok := extractSub2APITokensFromAuthResponse(reauthBody); ok && fromLogin != nil {
			upstreamUserID, found = *fromLogin, true
		}
	}
	if !found {
		slog.Error("account delete cannot resolve upstream user id", "user_id", profile.ID, "email", profile.Email)
		writeError(w, http.StatusInternalServerError, "failed to resolve upstream account")
		return
	}

	// Authoritative delete first: local rows are only purged once upstream
	// confirms (2xx, or 404 = already gone).
	if err := r.proxyClient.DoAdminJSON(ctx, proxy.AdminRequest{
		Method: http.MethodDelete,
		Path:   fmt.Sprintf("/api/v1/admin/users/%d", upstreamUserID),
	}, nil); err != nil {
		var apiErr *proxy.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			slog.Info("account already deleted upstream", "user_id", profile.ID, "upstream_user_id", upstreamUserID)
		} else {
			slog.Warn("sub2api account delete failed", "user_id", profile.ID, "upstream_user_id", upstreamUserID, "error", err)
			writeError(w, http.StatusBadGateway, "failed to delete upstream account")
			return
		}
	}

	if err := r.purgeLocalAccountData(req.Context(), profile.ID); err != nil {
		slog.Error("local account purge failed", "user_id", profile.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to purge local account")
		return
	}

	// Best-effort mirror purge; its failure never fails the deletion.
	r.purgePhoneServerAccount(req.Context(), profile.Email)

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// purgeLocalAccountData removes the user's sessions, sub2api token vault row
// and user row in a single transaction (children first so foreign keys stay
// happy on either dialect).
func (r *routes) purgeLocalAccountData(ctx context.Context, userID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin account purge transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range []struct {
		name string
		sql  string
	}{
		{"sessions", `DELETE FROM als_sessions WHERE user_id = ?;`},
		{"sub2api token vault", `DELETE FROM als_sub2api_auth_tokens WHERE user_id = ?;`},
		{"user", `DELETE FROM als_users WHERE id = ?;`},
	} {
		if _, err := tx.ExecContext(ctx, db.Rebind(r.sqlDialect, stmt.sql), userID); err != nil {
			return fmt.Errorf("purge %s: %w", stmt.name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit account purge: %w", err)
	}
	return nil
}

// purgePhoneServerAccount deletes the mirrored account on the companion phone
// platform server. It is skipped entirely when the admin URL/token are not
// configured, and any failure is logged and swallowed: the authoritative
// upstream deletion has already succeeded at this point.
func (r *routes) purgePhoneServerAccount(ctx context.Context, email string) {
	baseURL := strings.TrimRight(strings.TrimSpace(r.phoneServerAdminURL), "/")
	token := strings.TrimSpace(r.phoneServerAdminToken)
	if baseURL == "" || token == "" {
		return
	}

	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return
	}

	purgeCtx, cancel := context.WithTimeout(ctx, accountDeleteUpstreamTimeout)
	defer cancel()

	phoneServerUserID, err := findPhoneServerUserByEmail(purgeCtx, baseURL, token, email)
	if err != nil {
		slog.Warn("phone server account lookup failed", "email", email, "error", err)
		return
	}
	if phoneServerUserID == "" {
		slog.Info("phone server has no matching account", "email", email)
		return
	}

	// purgeData=true opts the phone server into wiping the account's business
	// rows (sessions, voice-transcription records) too, not just the account —
	// self-service deletion must remove personal data, per App Store 5.1.1(v).
	deleteReq, err := http.NewRequestWithContext(purgeCtx, http.MethodDelete, baseURL+"/api/admin/users/"+url.PathEscape(phoneServerUserID)+"?purgeData=true", nil)
	if err != nil {
		slog.Warn("build phone server delete request failed", "email", email, "error", err)
		return
	}
	setPhoneServerAdminHeaders(deleteReq.Header, token)
	resp, err := phoneServerHTTPClient.Do(deleteReq)
	if err != nil {
		slog.Warn("phone server account delete failed", "email", email, "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		slog.Info("phone server account already gone", "email", email)
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		slog.Warn("phone server account delete failed", "email", email, "status", resp.StatusCode, "body", strings.TrimSpace(string(body)))
		return
	}
	slog.Info("phone server mirror account deleted", "email", email)
}

// findPhoneServerUserByEmail resolves the phone server's user id for an email,
// or "" when no account matches. The phone server's admin console requires
// both the x-admin-token credential and the x-admin-console flag.
func findPhoneServerUserByEmail(ctx context.Context, baseURL, token, email string) (string, error) {
	listReq, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/admin/users", nil)
	if err != nil {
		return "", fmt.Errorf("build phone server list request: %w", err)
	}
	setPhoneServerAdminHeaders(listReq.Header, token)

	resp, err := phoneServerHTTPClient.Do(listReq)
	if err != nil {
		return "", fmt.Errorf("list phone server users: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("list phone server users: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var listResponse struct {
		Users []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResponse); err != nil {
		return "", fmt.Errorf("decode phone server users: %w", err)
	}
	for _, entry := range listResponse.Users {
		if strings.EqualFold(strings.TrimSpace(entry.Email), email) {
			return strings.TrimSpace(entry.ID), nil
		}
	}
	return "", nil
}

func setPhoneServerAdminHeaders(header http.Header, token string) {
	header.Set("Accept", "application/json")
	header.Set("x-admin-token", token)
	header.Set("x-admin-console", "1")
}
