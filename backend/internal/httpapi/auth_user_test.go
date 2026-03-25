package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"ai-api-portal/backend/internal/auth"
	"ai-api-portal/backend/internal/user"
)

type testMailSender struct {
	sent []sentMail
}

type sentMail struct {
	ToEmail string
	Subject string
	Body    string
}

func (s *testMailSender) Send(_ context.Context, toEmail, subject, body string) error {
	s.sent = append(s.sent, sentMail{ToEmail: toEmail, Subject: subject, Body: body})
	return nil
}

func TestUserMeAuthenticatedAndUnauthenticated(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server, database := setupTestServer(t)
	userID := createUser(t, ctx, database, "me-httpapi@example.com", "Me HTTP API", "user")

	unauthReq := httptest.NewRequest(http.MethodGet, "/user/me", nil)
	unauthRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /user/me status = %d, body=%s", unauthRec.Code, unauthRec.Body.String())
	}

	authReq := makeAuthenticatedRequest(t, ctx, database, http.MethodGet, "/user/me", nil, userID)
	authRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(authRec, authReq)

	if authRec.Code != http.StatusOK {
		t.Fatalf("authenticated /user/me status = %d, body=%s", authRec.Code, authRec.Body.String())
	}

	var profile struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(authRec.Body).Decode(&profile); err != nil {
		t.Fatalf("decode /user/me response: %v", err)
	}
	if profile.ID != userID || profile.Email != "me-httpapi@example.com" || profile.Name != "Me HTTP API" || profile.Role != "user" {
		t.Fatalf("unexpected /user/me payload: %+v", profile)
	}
}

func TestUpdateMeSuccessAndEmailTaken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server, database := setupTestServer(t)

	userID := createUser(t, ctx, database, "update-httpapi@example.com", "Update HTTP API", "user")
	_ = createUser(t, ctx, database, "taken-httpapi@example.com", "Taken HTTP API", "user")

	successReq := makeAuthenticatedRequest(t, ctx, database, http.MethodPut, "/user/me", []byte(`{"name":"Updated Name","email":"updated-httpapi@example.com"}`), userID)
	successRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(successRec, successReq)

	if successRec.Code != http.StatusOK {
		t.Fatalf("update /user/me success status = %d, body=%s", successRec.Code, successRec.Body.String())
	}

	var updated struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(successRec.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update success response: %v", err)
	}
	if updated.ID != userID || updated.Name != "Updated Name" || updated.Email != "updated-httpapi@example.com" {
		t.Fatalf("unexpected updated payload: %+v", updated)
	}

	conflictReq := makeAuthenticatedRequest(t, ctx, database, http.MethodPut, "/user/me", []byte(`{"name":"Another Name","email":"taken-httpapi@example.com"}`), userID)
	conflictRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(conflictRec, conflictReq)

	if conflictRec.Code != http.StatusConflict {
		t.Fatalf("update /user/me conflict status = %d, body=%s", conflictRec.Code, conflictRec.Body.String())
	}
}

func TestChangePasswordSuccessAndWrongOldPassword(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server, database := setupTestServer(t)

	oldPassword := "OldPassword#123"
	hash, err := user.HashPassword(oldPassword)
	if err != nil {
		t.Fatalf("hash old password: %v", err)
	}

	userID := createUserWithPasswordHash(t, ctx, database, "password-httpapi@example.com", "Password HTTP API", "user", hash)

	changeReq := makeAuthenticatedRequest(t, ctx, database, http.MethodPut, "/user/password", []byte(`{"old_password":"OldPassword#123","new_password":"NewPassword#789"}`), userID)
	changeRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(changeRec, changeReq)

	if changeRec.Code != http.StatusOK {
		t.Fatalf("change password success status = %d, body=%s", changeRec.Code, changeRec.Body.String())
	}

	wrongReq := makeAuthenticatedRequest(t, ctx, database, http.MethodPut, "/user/password", []byte(`{"old_password":"not-correct","new_password":"AnotherPassword#111"}`), userID)
	wrongRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(wrongRec, wrongReq)

	if wrongRec.Code != http.StatusUnauthorized {
		t.Fatalf("change password wrong old password status = %d, body=%s", wrongRec.Code, wrongRec.Body.String())
	}

	userSvc := user.NewService(database)
	if _, err := userSvc.Login(ctx, "password-httpapi@example.com", "NewPassword#789"); err != nil {
		t.Fatalf("login with new password: %v", err)
	}

	if _, err := userSvc.Login(ctx, "password-httpapi@example.com", "OldPassword#123"); err == nil {
		t.Fatalf("expected old password login to fail")
	}
}

func TestSetInitialPasswordSuccessAndAlreadySet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server, database := setupTestServer(t)

	userID := createUser(t, ctx, database, "set-initial-httpapi@example.com", "Set Initial HTTP API", "user")

	setReq := makeAuthenticatedRequest(t, ctx, database, http.MethodPost, "/user/password", []byte(`{"new_password":"InitialPassword#123"}`), userID)
	setRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(setRec, setReq)

	if setRec.Code != http.StatusOK {
		t.Fatalf("set initial password status = %d, body=%s", setRec.Code, setRec.Body.String())
	}

	var setPayload struct {
		Set bool `json:"set"`
	}
	if err := json.NewDecoder(setRec.Body).Decode(&setPayload); err != nil {
		t.Fatalf("decode set initial password response: %v", err)
	}
	if !setPayload.Set {
		t.Fatalf("expected set=true, got false")
	}

	userSvc := user.NewService(database)
	if _, err := userSvc.Login(ctx, "set-initial-httpapi@example.com", "InitialPassword#123"); err != nil {
		t.Fatalf("login with initial password: %v", err)
	}

	existingHash, err := user.HashPassword("ExistingPassword#456")
	if err != nil {
		t.Fatalf("hash existing password: %v", err)
	}
	existingUserID := createUserWithPasswordHash(t, ctx, database, "already-set-httpapi@example.com", "Already Set HTTP API", "user", existingHash)

	alreadySetReq := makeAuthenticatedRequest(t, ctx, database, http.MethodPost, "/user/password", []byte(`{"new_password":"AnotherPassword#789"}`), existingUserID)
	alreadySetRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(alreadySetRec, alreadySetReq)

	if alreadySetRec.Code != http.StatusConflict {
		t.Fatalf("set initial password for already-set user status = %d, body=%s", alreadySetRec.Code, alreadySetRec.Body.String())
	}
}

func TestVerifyEmailAndDomainRestrictedRegisterViaService(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	userSvc := user.NewServiceWithOptions(database, user.ServiceOptions{AllowedEmailDomains: []string{"example.com"}})
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{UserService: userSvc})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	registerResult, err := userSvc.Register(ctx, "new-user@example.com", "New User", "Password#123")
	if err != nil {
		t.Fatalf("register via service: %v", err)
	}
	if !registerResult.RequireEmailVerification {
		t.Fatalf("expected register result require_email_verification=true")
	}

	if _, err := userSvc.Login(ctx, "new-user@example.com", "Password#123"); err != user.ErrEmailNotVerified {
		t.Fatalf("expected login before verify to fail with ErrEmailNotVerified, got %v", err)
	}

	var code string
	err = database.QueryRowContext(ctx, `
		SELECT code FROM email_verification_tokens evt
		JOIN users u ON u.id = evt.user_id
		WHERE u.email = ?
		ORDER BY evt.id DESC
		LIMIT 1;
	`, "new-user@example.com").Scan(&code)
	if err != nil {
		t.Fatalf("query verification code: %v", err)
	}

	verifyReq := httptest.NewRequest(http.MethodPost, "/auth/verify-email", bytes.NewReader([]byte(`{"email":"new-user@example.com","code":"`+code+`"}`)))
	verifyRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify email status = %d, body=%s", verifyRec.Code, verifyRec.Body.String())
	}

	if _, err := userSvc.Login(ctx, "new-user@example.com", "Password#123"); err != nil {
		t.Fatalf("login after verify via service: %v", err)
	}

	if _, err := userSvc.Register(ctx, "blocked@forbidden.com", "Blocked", "Password#123"); err != user.ErrInvalidEmailDomain {
		t.Fatalf("expected blocked domain register to fail with ErrInvalidEmailDomain, got %v", err)
	}
}

func TestRegisterWithoutEmailVerificationAllowsImmediateLoginViaService(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	requireEmailVerification := false
	userSvc := user.NewServiceWithOptions(database, user.ServiceOptions{RequireEmailVerification: &requireEmailVerification})
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{UserService: userSvc})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	registerResult, err := userSvc.Register(ctx, "no-verify@example.com", "No Verify", "Password#123")
	if err != nil {
		t.Fatalf("register via service: %v", err)
	}
	if registerResult.RequireEmailVerification {
		t.Fatalf("expected register result require_email_verification=false")
	}
	if !registerResult.EmailVerified {
		t.Fatalf("expected register result email_verified=true when verification disabled")
	}

	var tokens int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_verification_tokens;`).Scan(&tokens); err != nil {
		t.Fatalf("count verification tokens: %v", err)
	}
	if tokens != 0 {
		t.Fatalf("expected no email verification tokens, got %d", tokens)
	}

	if _, err := userSvc.Login(ctx, "no-verify@example.com", "Password#123"); err != nil {
		t.Fatalf("login via service: %v", err)
	}
}

func TestLoginAllowsExistingUnverifiedUserWhenEmailVerificationDisabled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)

	password := "Password#123"
	passwordHash, err := user.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO users(email, name, role, password_hash, email_verified)
		VALUES (?, ?, 'user', ?, 0);
	`, "existing-unverified@example.com", "Existing Unverified", passwordHash); err != nil {
		t.Fatalf("insert unverified user: %v", err)
	}

	requireEmailVerification := false
	userSvc := user.NewServiceWithOptions(database, user.ServiceOptions{RequireEmailVerification: &requireEmailVerification})
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{UserService: userSvc})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	if _, err := userSvc.Login(ctx, "existing-unverified@example.com", "Password#123"); err != nil {
		t.Fatalf("login via service: %v", err)
	}
}

func TestWalletRedeemAndProfileCRUD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server, database := setupTestServer(t)

	userID := createUser(t, ctx, database, "wallet-user@example.com", "Wallet User", "user")

	if _, err := database.ExecContext(ctx, `INSERT INTO user_wallets(user_id, balance_micros, currency) VALUES (?, 0, 'CNY');`, userID); err != nil {
		t.Fatalf("insert wallet: %v", err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO recharge_cards(card_code, amount_micros, currency) VALUES ('CARD-OK-100', 1000000, 'CNY');`); err != nil {
		t.Fatalf("insert recharge card: %v", err)
	}

	getWalletReq := makeAuthenticatedRequest(t, ctx, database, http.MethodGet, "/wallet", nil, userID)
	getWalletRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(getWalletRec, getWalletReq)
	if getWalletRec.Code != http.StatusOK {
		t.Fatalf("get wallet status = %d, body=%s", getWalletRec.Code, getWalletRec.Body.String())
	}

	redeemReq := makeAuthenticatedRequest(t, ctx, database, http.MethodPost, "/wallet/redeem", []byte(`{"card_code":"CARD-OK-100"}`), userID)
	redeemRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(redeemRec, redeemReq)
	if redeemRec.Code != http.StatusOK {
		t.Fatalf("redeem card status = %d, body=%s", redeemRec.Code, redeemRec.Body.String())
	}

	redeemAgainReq := makeAuthenticatedRequest(t, ctx, database, http.MethodPost, "/wallet/redeem", []byte(`{"card_code":"CARD-OK-100"}`), userID)
	redeemAgainRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(redeemAgainRec, redeemAgainReq)
	if redeemAgainRec.Code != http.StatusConflict {
		t.Fatalf("redeem same card again status = %d, body=%s", redeemAgainRec.Code, redeemAgainRec.Body.String())
	}

	txReq := makeAuthenticatedRequest(t, ctx, database, http.MethodGet, "/wallet/transactions", nil, userID)
	txRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(txRec, txReq)
	if txRec.Code != http.StatusOK {
		t.Fatalf("list wallet transactions status = %d, body=%s", txRec.Code, txRec.Body.String())
	}

	createProfileReq := makeAuthenticatedRequest(t, ctx, database, http.MethodPost, "/profiles", []byte(`{"profile_name":"main-json","profile_type":"chat","is_active":true,"content_format":"json","content_text":"{\"model\":\"gpt-4.1\"}"}`), userID)
	createProfileRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(createProfileRec, createProfileReq)
	if createProfileRec.Code != http.StatusCreated {
		t.Fatalf("create profile status = %d, body=%s", createProfileRec.Code, createProfileRec.Body.String())
	}

	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(createProfileRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created profile: %v", err)
	}
	if created.ID <= 0 {
		t.Fatalf("expected positive profile id, got %d", created.ID)
	}

	badProfileReq := makeAuthenticatedRequest(t, ctx, database, http.MethodPost, "/profiles", []byte(`{"profile_name":"bad-json","profile_type":"chat","is_active":false,"content_format":"json","content_text":"{bad json}"}`), userID)
	badProfileRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(badProfileRec, badProfileReq)
	if badProfileRec.Code != http.StatusBadRequest {
		t.Fatalf("create bad profile status = %d, body=%s", badProfileRec.Code, badProfileRec.Body.String())
	}

	listProfilesReq := makeAuthenticatedRequest(t, ctx, database, http.MethodGet, "/profiles", nil, userID)
	listProfilesRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(listProfilesRec, listProfilesReq)
	if listProfilesRec.Code != http.StatusOK {
		t.Fatalf("list profiles status = %d, body=%s", listProfilesRec.Code, listProfilesRec.Body.String())
	}

	getProfileReq := makeAuthenticatedRequest(t, ctx, database, http.MethodGet, "/profiles/"+strconv.FormatInt(created.ID, 10), nil, userID)
	getProfileRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(getProfileRec, getProfileReq)
	if getProfileRec.Code != http.StatusOK {
		t.Fatalf("get profile status = %d, body=%s", getProfileRec.Code, getProfileRec.Body.String())
	}

	updateProfileReq := makeAuthenticatedRequest(t, ctx, database, http.MethodPut, "/profiles/"+strconv.FormatInt(created.ID, 10), []byte(`{"profile_name":"main-yaml","profile_type":"chat","is_active":true,"content_format":"yaml","content_text":"model: gpt-4.1\ntemperature: 0.2"}`), userID)
	updateProfileRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(updateProfileRec, updateProfileReq)
	if updateProfileRec.Code != http.StatusOK {
		t.Fatalf("update profile status = %d, body=%s", updateProfileRec.Code, updateProfileRec.Body.String())
	}

	deleteProfileReq := makeAuthenticatedRequest(t, ctx, database, http.MethodDelete, "/profiles/"+strconv.FormatInt(created.ID, 10), nil, userID)
	deleteProfileRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(deleteProfileRec, deleteProfileReq)
	if deleteProfileRec.Code != http.StatusOK {
		t.Fatalf("delete profile status = %d, body=%s", deleteProfileRec.Code, deleteProfileRec.Body.String())
	}
}

func TestDeleteSessionSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server, database := setupTestServer(t)
	userID := createUser(t, ctx, database, "logout-httpapi@example.com", "Logout HTTP API", "user")

	logoutReq := makeAuthenticatedRequest(t, ctx, database, http.MethodDelete, "/session", nil, userID)
	logoutRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusOK {
		t.Fatalf("delete /session status = %d, body=%s", logoutRec.Code, logoutRec.Body.String())
	}

	var payload struct {
		Revoked bool `json:"revoked"`
	}
	if err := json.NewDecoder(logoutRec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode logout response: %v", err)
	}
	if !payload.Revoked {
		t.Fatalf("expected revoked=true, got false")
	}

	meAfterLogoutRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(meAfterLogoutRec, logoutReq)
	if meAfterLogoutRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked session to be unauthorized, got status=%d body=%s", meAfterLogoutRec.Code, meAfterLogoutRec.Body.String())
	}
}

func TestListSessions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server, database := setupTestServer(t)
	userID := createUser(t, ctx, database, "sessions-httpapi@example.com", "Sessions HTTP API", "user")

	_, secondSessionTokenHash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("generate second session token: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO sessions(user_id, token_hash, expires_at)
		VALUES (?, ?, ?);
	`, userID, secondSessionTokenHash, time.Now().UTC().Add(24*time.Hour).Format("2006-01-02 15:04:05")); err != nil {
		t.Fatalf("insert second session: %v", err)
	}

	listReq := makeAuthenticatedRequest(t, ctx, database, http.MethodGet, "/sessions", nil, userID)
	listRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("get /sessions status = %d, body=%s", listRec.Code, listRec.Body.String())
	}

	var payload struct {
		Sessions []struct {
			ID        int64  `json:"id"`
			CreatedAt string `json:"created_at"`
			ExpiresAt string `json:"expires_at"`
			IsRevoked bool   `json:"is_revoked"`
		} `json:"sessions"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode /sessions response: %v", err)
	}
	if len(payload.Sessions) < 2 {
		t.Fatalf("expected at least 2 sessions, got %d", len(payload.Sessions))
	}
	for _, s := range payload.Sessions {
		if s.ID <= 0 {
			t.Fatalf("expected positive session id, got %d", s.ID)
		}
		if s.CreatedAt == "" || s.ExpiresAt == "" {
			t.Fatalf("expected non-empty created_at/expires_at, got %+v", s)
		}
	}
}

func setupTestServer(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()

	database := setupTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, database)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server, database
}

func TestForgotPasswordSendsVerificationCodesAndResetWorks(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	mailCapture := &testMailSender{}
	userSvc := user.NewServiceWithMailSender(database, mailCapture)

	mux := http.NewServeMux()
	RegisterRoutesWithUserService(mux, database, userSvc)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	registerResult, err := userSvc.Register(ctx, "mail-user@example.com", "Mail User", "Password#123")
	if err != nil {
		t.Fatalf("register via service: %v", err)
	}
	if !registerResult.RequireEmailVerification {
		t.Fatalf("expected register result require_email_verification=true")
	}

	if len(mailCapture.sent) != 1 {
		t.Fatalf("expected 1 sent email after register, got %d", len(mailCapture.sent))
	}

	forgotReq := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", bytes.NewReader([]byte(`{"email":"mail-user@example.com"}`)))
	forgotRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(forgotRec, forgotReq)
	if forgotRec.Code != http.StatusOK {
		t.Fatalf("forgot password status = %d, body=%s", forgotRec.Code, forgotRec.Body.String())
	}

	if len(mailCapture.sent) != 2 {
		t.Fatalf("expected 2 sent emails after forgot password, got %d", len(mailCapture.sent))
	}

	var resetCode string
	err = database.QueryRowContext(ctx, `
		SELECT prt.code
		FROM password_reset_tokens prt
		JOIN users u ON u.id = prt.user_id
		WHERE u.email = ?
		ORDER BY prt.id DESC
		LIMIT 1;
	`, "mail-user@example.com").Scan(&resetCode)
	if err != nil {
		t.Fatalf("query password reset code: %v", err)
	}

	resetReq := httptest.NewRequest(http.MethodPost, "/auth/reset-password", bytes.NewReader([]byte(`{"email":"mail-user@example.com","code":"`+resetCode+`","new_password":"NewPassword#789"}`)))
	resetRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(resetRec, resetReq)
	if resetRec.Code != http.StatusOK {
		t.Fatalf("reset password status = %d, body=%s", resetRec.Code, resetRec.Body.String())
	}

	if _, err := userSvc.Login(ctx, "mail-user@example.com", "NewPassword#789"); err != user.ErrEmailNotVerified {
		t.Fatalf("expected login after reset before verify to fail with ErrEmailNotVerified, got %v", err)
	}

	var verifyCode string
	err = database.QueryRowContext(ctx, `
		SELECT evt.code
		FROM email_verification_tokens evt
		JOIN users u ON u.id = evt.user_id
		WHERE u.email = ?
		ORDER BY evt.id DESC
		LIMIT 1;
	`, "mail-user@example.com").Scan(&verifyCode)
	if err != nil {
		t.Fatalf("query verification code: %v", err)
	}

	verifyReq := httptest.NewRequest(http.MethodPost, "/auth/verify-email", bytes.NewReader([]byte(`{"email":"mail-user@example.com","code":"`+verifyCode+`"}`)))
	verifyRec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify email status = %d, body=%s", verifyRec.Code, verifyRec.Body.String())
	}

	if _, err := userSvc.Login(ctx, "mail-user@example.com", "NewPassword#789"); err != nil {
		t.Fatalf("login after verify via service: %v", err)
	}
}

func createUser(t *testing.T, ctx context.Context, database *sql.DB, email, name, role string) int64 {
	t.Helper()

	result, err := database.ExecContext(ctx, `
		INSERT INTO users(email, name, role)
		VALUES (?, ?, ?);
	`, email, name, role)
	if err != nil {
		t.Fatalf("createUser insert error = %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("createUser LastInsertId error = %v", err)
	}

	return id
}

func createUserWithPasswordHash(t *testing.T, ctx context.Context, database *sql.DB, email, name, role, passwordHash string) int64 {
	t.Helper()

	result, err := database.ExecContext(ctx, `
		INSERT INTO users(email, name, role, password_hash)
		VALUES (?, ?, ?, ?);
	`, email, name, role, passwordHash)
	if err != nil {
		t.Fatalf("createUserWithPasswordHash insert error = %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("createUserWithPasswordHash LastInsertId error = %v", err)
	}

	return id
}

func makeAuthenticatedRequest(t *testing.T, ctx context.Context, database *sql.DB, method, path string, body []byte, userID int64) *http.Request {
	t.Helper()

	plaintextToken, tokenHash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("new session token: %v", err)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO sessions(user_id, token_hash, expires_at)
		VALUES (?, ?, ?);
	`, userID, tokenHash, time.Now().UTC().Add(24*time.Hour).Format("2006-01-02 15:04:05")); err != nil {
		t.Fatalf("insert session for authenticated request: %v", err)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	setBearerAuth(req, plaintextToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req
}
