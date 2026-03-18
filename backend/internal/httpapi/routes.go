package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-api-portal/backend/internal/apikey"
	"ai-api-portal/backend/internal/auth"
	"ai-api-portal/backend/internal/user"
)

type routes struct {
	db                   *sql.DB
	apiKey               *apikey.Service
	userSvc              *user.Service
	adminBootstrapSecret string
}

type RoutesOptions struct {
	UserService          *user.Service
	AdminBootstrapSecret string
}

type createUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type createUserResponse struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	SessionToken string `json:"session_token"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type verifyEmailRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Email       string `json:"email"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
}

type loginUserResponse struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type loginResponse struct {
	User         loginUserResponse `json:"user"`
	SessionToken string            `json:"session_token"`
}

type updateProfileRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type setInitialPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

type redeemCardRequest struct {
	CardCode string `json:"card_code"`
}

type profileConfigRequest struct {
	ProfileName   string `json:"profile_name"`
	ProfileType   string `json:"profile_type"`
	IsActive      bool   `json:"is_active"`
	ContentFormat string `json:"content_format"`
	ContentText   string `json:"content_text"`
}

type walletResponse struct {
	Wallet user.Wallet `json:"wallet"`
}

type walletTransactionsResponse struct {
	Transactions []user.WalletTransaction `json:"transactions"`
}

type listProfileConfigsResponse struct {
	Profiles []user.ProfileConfig `json:"profiles"`
}

type sessionResponse struct {
	ID        int64  `json:"id"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
	IsRevoked bool   `json:"is_revoked"`
}

type listSessionsResponse struct {
	Sessions []sessionResponse `json:"sessions"`
}

type revokeSessionResponse struct {
	Revoked bool `json:"revoked"`
}

type createAPIKeyRequest struct {
	Label string `json:"label"`
}

type createAPIKeyResponse struct {
	ID        int64  `json:"id"`
	Label     string `json:"label"`
	APIKey    string `json:"api_key"`
	CreatedAt string `json:"created_at"`
}

type revokeAPIKeyResponse struct {
	Revoked bool `json:"revoked"`
}

type publicTierItemResponse struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Unit          string `json:"unit"`
	IncludedUnits int64  `json:"included_units"`
}

type publicTierResponse struct {
	Code         string                   `json:"code"`
	Name         string                   `json:"name"`
	DefaultItems []publicTierItemResponse `json:"default_items"`
}

type listPublicTiersResponse struct {
	Tiers []publicTierResponse `json:"tiers"`
}

type publicEstimateRequest struct {
	TierCode string `json:"tier_code"`
}

type publicEstimateItemResponse struct {
	Code               string `json:"code"`
	Name               string `json:"name"`
	Unit               string `json:"unit"`
	IncludedUnits      int64  `json:"included_units"`
	PricePerUnitMicros int64  `json:"price_per_unit_micros,omitempty"`
	LineTotalMicros    int64  `json:"line_total_micros,omitempty"`
	Currency           string `json:"currency,omitempty"`
	MissingPrice       bool   `json:"missing_price"`
}

type publicEstimateResponse struct {
	TierCode         string                       `json:"tier_code"`
	TierName         string                       `json:"tier_name"`
	Currency         string                       `json:"currency,omitempty"`
	TotalPriceMicros int64                        `json:"total_price_micros"`
	Items            []publicEstimateItemResponse `json:"items"`
}

type subscriptionOverrideRequest struct {
	ServiceItemCode string `json:"service_item_code"`
	IncludedUnits   int64  `json:"included_units"`
}

type createSubscriptionRequest struct {
	TierCode  string                        `json:"tier_code"`
	Overrides []subscriptionOverrideRequest `json:"overrides"`
}

type subscriptionQuotaResponse struct {
	ServiceItemCode string `json:"service_item_code"`
	ServiceItemName string `json:"service_item_name"`
	Unit            string `json:"unit"`
	IncludedUnits   int64  `json:"included_units"`
}

type subscriptionResponse struct {
	TierCode string                      `json:"tier_code"`
	TierName string                      `json:"tier_name"`
	Quotas   []subscriptionQuotaResponse `json:"quotas"`
}

type getSubscriptionResponse struct {
	Subscription subscriptionResponse `json:"subscription"`
}

type adminSetUnitPriceRequest struct {
	ServiceItemCode    string `json:"service_item_code"`
	TierCode           string `json:"tier_code"`
	PricePerUnitMicros int64  `json:"price_per_unit_micros"`
	Currency           string `json:"currency"`
}

type aiRequestPayload struct {
	ServiceItemCode string `json:"service_item_code"`
	Quantity        int64  `json:"quantity"`
}

type aiRequestResponse struct {
	Allowed        bool  `json:"allowed"`
	IncludedUnits  int64 `json:"included_units"`
	UsedUnits      int64 `json:"used_units"`
	RemainingUnits int64 `json:"remaining_units"`
}

type adminUnitPriceResponse struct {
	ServiceItemCode    string  `json:"service_item_code"`
	TierCode           *string `json:"tier_code,omitempty"`
	PricePerUnitMicros int64   `json:"price_per_unit_micros"`
	Currency           string  `json:"currency"`
	EffectiveFrom      string  `json:"effective_from"`
}

type listAdminUnitPricesResponse struct {
	UnitPrices []adminUnitPriceResponse `json:"unit_prices"`
}

type deactivateUnitPriceResponse struct {
	Deactivated bool `json:"deactivated"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func RegisterRoutes(mux *http.ServeMux, database *sql.DB) {
	RegisterRoutesWithOptions(mux, database, RoutesOptions{})
}

func RegisterRoutesWithUserService(mux *http.ServeMux, database *sql.DB, userSvc *user.Service) {
	RegisterRoutesWithOptions(mux, database, RoutesOptions{UserService: userSvc})
}

func RegisterRoutesWithOptions(mux *http.ServeMux, database *sql.DB, opts RoutesOptions) {
	userSvc := opts.UserService
	if userSvc == nil {
		userSvc = user.NewService(database)
	}
	r := &routes{
		db:                   database,
		apiKey:               apikey.NewService(database),
		userSvc:              userSvc,
		adminBootstrapSecret: strings.TrimSpace(opts.AdminBootstrapSecret),
	}
	authenticated := auth.RequireUser(database)

	mux.HandleFunc("POST /users", r.handleCreateUser)
	mux.HandleFunc("POST /auth/register", r.handleRegister)
	mux.HandleFunc("POST /auth/verify-email", r.handleVerifyEmail)
	mux.HandleFunc("POST /auth/forgot-password", r.handleForgotPassword)
	mux.HandleFunc("POST /auth/reset-password", r.handleResetPassword)
	mux.HandleFunc("POST /auth/login", r.handleLogin)
	mux.Handle("GET /user/me", authenticated(http.HandlerFunc(r.handleGetMe)))
	mux.Handle("PUT /user/me", authenticated(http.HandlerFunc(r.handleUpdateMe)))
	mux.Handle("PUT /user/password", authenticated(http.HandlerFunc(r.handleChangePassword)))
	mux.Handle("POST /user/password", authenticated(http.HandlerFunc(r.handleSetInitialPassword)))
	mux.Handle("GET /wallet", authenticated(http.HandlerFunc(r.handleGetWallet)))
	mux.Handle("GET /wallet/transactions", authenticated(http.HandlerFunc(r.handleListWalletTransactions)))
	mux.Handle("POST /wallet/redeem", authenticated(http.HandlerFunc(r.handleRedeemCard)))
	mux.Handle("POST /profiles", authenticated(http.HandlerFunc(r.handleCreateProfileConfig)))
	mux.Handle("GET /profiles", authenticated(http.HandlerFunc(r.handleListProfileConfigs)))
	mux.Handle("GET /profiles/{id}", authenticated(http.HandlerFunc(r.handleGetProfileConfig)))
	mux.Handle("PUT /profiles/{id}", authenticated(http.HandlerFunc(r.handleUpdateProfileConfig)))
	mux.Handle("DELETE /profiles/{id}", authenticated(http.HandlerFunc(r.handleDeleteProfileConfig)))
	mux.Handle("DELETE /session", authenticated(http.HandlerFunc(r.handleLogout)))
	mux.Handle("GET /sessions", authenticated(http.HandlerFunc(r.handleListSessions)))
	mux.HandleFunc("GET /public/tiers", r.handlePublicTiers)
	mux.HandleFunc("POST /public/estimate", r.handlePublicEstimate)
	mux.Handle("POST /subscription", authenticated(http.HandlerFunc(r.handleCreateSubscription)))
	mux.Handle("GET /subscription", authenticated(http.HandlerFunc(r.handleGetSubscription)))
	mux.Handle("POST /api-keys", authenticated(http.HandlerFunc(r.handleCreateAPIKey)))
	mux.Handle("DELETE /api-keys/{id}", authenticated(http.HandlerFunc(r.handleRevokeAPIKey)))
	mux.Handle("GET /admin/unit-prices", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListUnitPrices))))
	mux.Handle("PUT /admin/unit-prices", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminSetUnitPrice))))
	mux.Handle("DELETE /admin/unit-prices", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminDeactivateUnitPrice))))
	mux.HandleFunc("POST /api/ai/request", r.handleAIRequest)
}

func (r *routes) handleCreateUser(w http.ResponseWriter, req *http.Request) {
	var payload createUserRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.Email = strings.TrimSpace(payload.Email)
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Role = strings.TrimSpace(payload.Role)
	if payload.Email == "" || payload.Name == "" {
		writeError(w, http.StatusBadRequest, "email and name are required")
		return
	}
	if payload.Role == "" {
		payload.Role = "user"
	}
	if payload.Role != "user" && payload.Role != "admin" {
		writeError(w, http.StatusBadRequest, "role must be user or admin")
		return
	}

	if payload.Role == "admin" {
		expectedSecret := r.adminBootstrapSecret
		providedSecret := strings.TrimSpace(req.Header.Get("X-Admin-Bootstrap-Secret"))
		if expectedSecret == "" || providedSecret == "" || providedSecret != expectedSecret {
			writeError(w, http.StatusForbidden, "admin bootstrap secret required")
			return
		}
	}

	plaintextSessionToken, sessionTokenHash, err := auth.NewSessionToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user session")
		return
	}
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02 15:04:05")

	tx, err := r.db.BeginTx(req.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}
	defer func() { _ = tx.Rollback() }()

	const query = `INSERT INTO users(email, name, role) VALUES (?, ?, ?);`
	result, err := tx.ExecContext(req.Context(), query, payload.Email, payload.Name, payload.Role)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to create user: %v", err))
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read user id")
		return
	}

	if _, err := tx.ExecContext(req.Context(), `
		INSERT INTO sessions(user_id, token_hash, expires_at)
		VALUES (?, ?, ?);
	`, id, sessionTokenHash, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user session")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	writeJSON(w, http.StatusCreated, createUserResponse{ID: id, Email: payload.Email, Name: payload.Name, Role: payload.Role, SessionToken: plaintextSessionToken})
}

func (r *routes) handleRegister(w http.ResponseWriter, req *http.Request) {
	var payload registerRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	result, err := r.userSvc.Register(req.Context(), payload.Email, payload.Name, payload.Password)
	if errors.Is(err, user.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "email already taken")
		return
	}
	if errors.Is(err, user.ErrInvalidEmailDomain) {
		writeError(w, http.StatusForbidden, "email domain is not allowed")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (r *routes) handleVerifyEmail(w http.ResponseWriter, req *http.Request) {
	var payload verifyEmailRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	err := r.userSvc.VerifyEmail(req.Context(), payload.Email, payload.Code)
	if errors.Is(err, user.ErrInvalidCode) {
		writeError(w, http.StatusUnauthorized, "invalid verification code")
		return
	}
	if errors.Is(err, user.ErrCodeExpired) {
		writeError(w, http.StatusUnauthorized, "verification code expired")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify email")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"verified": true})
}

func (r *routes) handleForgotPassword(w http.ResponseWriter, req *http.Request) {
	var payload forgotPasswordRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if err := r.userSvc.RequestPasswordReset(req.Context(), payload.Email); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to request password reset")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"sent": true})
}

func (r *routes) handleResetPassword(w http.ResponseWriter, req *http.Request) {
	var payload resetPasswordRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	err := r.userSvc.ResetPasswordByCode(req.Context(), payload.Email, payload.Code, payload.NewPassword)
	if errors.Is(err, user.ErrInvalidCode) {
		writeError(w, http.StatusUnauthorized, "invalid verification code")
		return
	}
	if errors.Is(err, user.ErrCodeExpired) {
		writeError(w, http.StatusUnauthorized, "verification code expired")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reset password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"reset": true})
}

func (r *routes) handleLogin(w http.ResponseWriter, req *http.Request) {
	var payload loginRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.Email = strings.TrimSpace(payload.Email)
	payload.Password = strings.TrimSpace(payload.Password)
	if payload.Email == "" || payload.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	authUser, err := r.userSvc.Login(req.Context(), payload.Email, payload.Password)
	if errors.Is(err, user.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if errors.Is(err, user.ErrEmailNotVerified) {
		writeError(w, http.StatusForbidden, "email not verified")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to login")
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		User: loginUserResponse{
			ID:    authUser.ID,
			Email: authUser.Email,
			Name:  authUser.Name,
			Role:  authUser.Role,
		},
		SessionToken: authUser.SessionToken,
	})
}

func (r *routes) handleGetMe(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	profile, err := r.userSvc.GetProfile(req.Context(), authUser.ID)
	if errors.Is(err, user.ErrUserNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (r *routes) handleUpdateMe(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var payload updateProfileRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.Email = strings.TrimSpace(payload.Email)
	if payload.Name == "" || payload.Email == "" {
		writeError(w, http.StatusBadRequest, "name and email are required")
		return
	}

	if err := r.userSvc.UpdateProfile(req.Context(), authUser.ID, payload.Name, payload.Email); err != nil {
		if errors.Is(err, user.ErrEmailTaken) {
			writeError(w, http.StatusConflict, "email already taken")
			return
		}
		if errors.Is(err, user.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	profile, err := r.userSvc.GetProfile(req.Context(), authUser.ID)
	if errors.Is(err, user.ErrUserNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (r *routes) handleChangePassword(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var payload changePasswordRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.OldPassword = strings.TrimSpace(payload.OldPassword)
	payload.NewPassword = strings.TrimSpace(payload.NewPassword)
	if payload.OldPassword == "" || payload.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "old_password and new_password are required")
		return
	}

	if err := r.userSvc.ChangePassword(req.Context(), authUser.ID, payload.OldPassword, payload.NewPassword); err != nil {
		if errors.Is(err, user.ErrWrongPassword) || errors.Is(err, user.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "wrong current password")
			return
		}
		if errors.Is(err, user.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to change password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"changed": true})
}

func (r *routes) handleSetInitialPassword(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var payload setInitialPasswordRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.NewPassword = strings.TrimSpace(payload.NewPassword)
	if payload.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "new_password is required")
		return
	}

	if err := r.userSvc.SetInitialPassword(req.Context(), authUser.ID, payload.NewPassword); err != nil {
		if errors.Is(err, user.ErrPasswordAlreadySet) {
			writeError(w, http.StatusConflict, "password already set, use change password instead")
			return
		}
		if errors.Is(err, user.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to set password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"set": true})
}

func (r *routes) handleGetWallet(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	wallet, err := r.userSvc.GetWallet(req.Context(), authUser.ID)
	if errors.Is(err, user.ErrUserNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get wallet")
		return
	}

	writeJSON(w, http.StatusOK, walletResponse{Wallet: *wallet})
}

func (r *routes) handleRedeemCard(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var payload redeemCardRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	wallet, err := r.userSvc.RedeemCard(req.Context(), authUser.ID, payload.CardCode)
	if errors.Is(err, user.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "recharge card not found")
		return
	}
	if errors.Is(err, user.ErrCardAlreadyRedeemed) {
		writeError(w, http.StatusConflict, "recharge card already redeemed")
		return
	}
	if errors.Is(err, user.ErrCardExpired) {
		writeError(w, http.StatusConflict, "recharge card expired")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to redeem card")
		return
	}

	writeJSON(w, http.StatusOK, walletResponse{Wallet: *wallet})
}

func (r *routes) handleListWalletTransactions(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	limit := parseQueryLimit(req.URL.Query().Get("limit"), 20)
	txs, err := r.userSvc.ListWalletTransactions(req.Context(), authUser.ID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list wallet transactions")
		return
	}

	writeJSON(w, http.StatusOK, walletTransactionsResponse{Transactions: txs})
}

func (r *routes) handleCreateProfileConfig(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var payload profileConfigRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	cfg, err := r.userSvc.CreateProfileConfig(
		req.Context(),
		authUser.ID,
		payload.ProfileName,
		payload.ProfileType,
		payload.ContentFormat,
		payload.ContentText,
		payload.IsActive,
	)
	if errors.Is(err, user.ErrInvalidProfileData) {
		writeError(w, http.StatusBadRequest, "invalid profile data")
		return
	}
	if errors.Is(err, user.ErrProfileNameTaken) {
		writeError(w, http.StatusConflict, "profile name already taken")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create profile")
		return
	}

	writeJSON(w, http.StatusCreated, cfg)
}

func (r *routes) handleGetProfileConfig(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	profileID, err := parsePathID(req.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid profile id")
		return
	}

	cfg, err := r.userSvc.GetProfileConfig(req.Context(), authUser.ID, profileID)
	if errors.Is(err, user.ErrProfileNotFound) {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get profile")
		return
	}

	writeJSON(w, http.StatusOK, cfg)
}

func (r *routes) handleListProfileConfigs(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	profileType := strings.TrimSpace(req.URL.Query().Get("type"))
	profiles, err := r.userSvc.ListProfileConfigs(req.Context(), authUser.ID, profileType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list profiles")
		return
	}

	writeJSON(w, http.StatusOK, listProfileConfigsResponse{Profiles: profiles})
}

func (r *routes) handleUpdateProfileConfig(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	profileID, err := parsePathID(req.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid profile id")
		return
	}

	var payload profileConfigRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	cfg, err := r.userSvc.UpdateProfileConfig(
		req.Context(),
		authUser.ID,
		profileID,
		payload.ProfileName,
		payload.ProfileType,
		payload.ContentFormat,
		payload.ContentText,
		payload.IsActive,
	)
	if errors.Is(err, user.ErrProfileNotFound) {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}
	if errors.Is(err, user.ErrInvalidProfileData) {
		writeError(w, http.StatusBadRequest, "invalid profile data")
		return
	}
	if errors.Is(err, user.ErrProfileNameTaken) {
		writeError(w, http.StatusConflict, "profile name already taken")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	writeJSON(w, http.StatusOK, cfg)
}

func (r *routes) handleDeleteProfileConfig(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	profileID, err := parsePathID(req.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid profile id")
		return
	}

	err = r.userSvc.DeleteProfileConfig(req.Context(), authUser.ID, profileID)
	if errors.Is(err, user.ErrProfileNotFound) {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete profile")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (r *routes) handleLogout(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	token, err := extractBearerToken(req.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "missing bearer token")
		return
	}

	if err := r.userSvc.Logout(req.Context(), authUser.ID, token); err != nil {
		if errors.Is(err, user.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid or expired session")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to logout")
		return
	}

	writeJSON(w, http.StatusOK, revokeSessionResponse{Revoked: true})
}

func (r *routes) handleListSessions(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	sessions, err := r.userSvc.ListSessions(req.Context(), authUser.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}

	response := listSessionsResponse{Sessions: make([]sessionResponse, 0, len(sessions))}
	for _, item := range sessions {
		response.Sessions = append(response.Sessions, sessionResponse{
			ID:        item.ID,
			CreatedAt: item.CreatedAt.Format(time.RFC3339),
			ExpiresAt: item.ExpiresAt.Format(time.RFC3339),
			IsRevoked: item.RevokedAt != nil,
		})
	}

	writeJSON(w, http.StatusOK, response)
}

func (r *routes) handleCreateAPIKey(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var payload createAPIKeyRequest
	if req.Body != nil {
		_ = json.NewDecoder(req.Body).Decode(&payload)
	}

	created, err := r.apiKey.CreateKey(req.Context(), user.ID, payload.Label)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create api key")
		return
	}

	writeJSON(w, http.StatusCreated, createAPIKeyResponse{
		ID:        created.ID,
		Label:     created.Label,
		APIKey:    created.APIKey,
		CreatedAt: created.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (r *routes) handleRevokeAPIKey(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	rawID := req.PathValue("id")
	keyID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || keyID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid api key id")
		return
	}

	revoked, err := r.apiKey.RevokeKey(req.Context(), keyID, user.ID, user.Role == "admin")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke api key")
		return
	}
	if !revoked {
		writeError(w, http.StatusNotFound, "active api key not found")
		return
	}

	writeJSON(w, http.StatusOK, revokeAPIKeyResponse{Revoked: true})
}

func (r *routes) handleAdminListUnitPrices(w http.ResponseWriter, req *http.Request) {
	serviceItemCode := strings.TrimSpace(req.URL.Query().Get("service_item_code"))
	if serviceItemCode == "" {
		writeError(w, http.StatusBadRequest, "service_item_code is required")
		return
	}

	serviceItemID, err := r.lookupServiceItemID(req.Context(), serviceItemCode)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusBadRequest, "service_item_code not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list unit prices")
		return
	}

	tierCode := strings.TrimSpace(req.URL.Query().Get("tier_code"))
	hasTierFilter := tierCode != ""
	var tierID int64
	if hasTierFilter {
		tierID, err = r.lookupTierID(req.Context(), tierCode)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "tier_code not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list unit prices")
			return
		}
	}

	query := `
		SELECT
			si.code,
			t.code,
			up.price_per_unit_micros,
			up.currency,
			up.effective_from
		FROM unit_prices up
		JOIN service_items si ON si.id = up.service_item_id
		LEFT JOIN tiers t ON t.id = up.tier_id
		WHERE up.service_item_id = ?
			AND up.effective_to IS NULL
	`
	args := []any{serviceItemID}
	if hasTierFilter {
		query += " AND up.tier_id = ?"
		args = append(args, tierID)
	}
	query += " ORDER BY up.id ASC;"

	rows, err := r.db.QueryContext(req.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list unit prices")
		return
	}
	defer rows.Close()

	response := listAdminUnitPricesResponse{UnitPrices: make([]adminUnitPriceResponse, 0)}
	for rows.Next() {
		var (
			item          adminUnitPriceResponse
			tierCodeValue sql.NullString
		)
		if err := rows.Scan(&item.ServiceItemCode, &tierCodeValue, &item.PricePerUnitMicros, &item.Currency, &item.EffectiveFrom); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list unit prices")
			return
		}
		if tierCodeValue.Valid {
			tier := tierCodeValue.String
			item.TierCode = &tier
		}
		response.UnitPrices = append(response.UnitPrices, item)
	}

	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list unit prices")
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (r *routes) handleAdminSetUnitPrice(w http.ResponseWriter, req *http.Request) {
	var payload adminSetUnitPriceRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.ServiceItemCode = strings.TrimSpace(payload.ServiceItemCode)
	payload.TierCode = strings.TrimSpace(payload.TierCode)
	payload.Currency = strings.TrimSpace(payload.Currency)

	if payload.ServiceItemCode == "" {
		writeError(w, http.StatusBadRequest, "service_item_code is required")
		return
	}
	if payload.PricePerUnitMicros < 0 {
		writeError(w, http.StatusBadRequest, "price_per_unit_micros must be non-negative")
		return
	}

	currency, err := validateAndNormalizeCurrency(payload.Currency)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	serviceItemID, err := r.lookupServiceItemID(req.Context(), payload.ServiceItemCode)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusBadRequest, "service_item_code not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set unit price")
		return
	}

	hasTier := payload.TierCode != ""
	var tierID int64
	if hasTier {
		tierID, err = r.lookupTierID(req.Context(), payload.TierCode)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "tier_code not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to set unit price")
			return
		}
	}

	tx, err := r.db.BeginTx(req.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set unit price")
		return
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if hasTier {
		if _, err := tx.ExecContext(req.Context(), `
			UPDATE unit_prices
			SET effective_to = ?
			WHERE service_item_id = ?
				AND tier_id = ?
				AND effective_to IS NULL;
		`, now, serviceItemID, tierID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to set unit price")
			return
		}
	} else {
		if _, err := tx.ExecContext(req.Context(), `
			UPDATE unit_prices
			SET effective_to = ?
			WHERE service_item_id = ?
				AND tier_id IS NULL
				AND effective_to IS NULL;
		`, now, serviceItemID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to set unit price")
			return
		}
	}

	var tierArg any
	if hasTier {
		tierArg = tierID
	}
	if _, err := tx.ExecContext(req.Context(), `
		INSERT INTO unit_prices(service_item_id, tier_id, price_per_unit_micros, currency, effective_from)
		VALUES (?, ?, ?, ?, ?);
	`, serviceItemID, tierArg, payload.PricePerUnitMicros, currency, now); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to set unit price: %v", err))
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set unit price")
		return
	}

	response := adminUnitPriceResponse{
		ServiceItemCode:    payload.ServiceItemCode,
		PricePerUnitMicros: payload.PricePerUnitMicros,
		Currency:           currency,
		EffectiveFrom:      now,
	}
	if hasTier {
		tier := payload.TierCode
		response.TierCode = &tier
	}

	writeJSON(w, http.StatusCreated, response)
}

func (r *routes) handleAdminDeactivateUnitPrice(w http.ResponseWriter, req *http.Request) {
	serviceItemCode := strings.TrimSpace(req.URL.Query().Get("service_item_code"))
	if serviceItemCode == "" {
		writeError(w, http.StatusBadRequest, "service_item_code is required")
		return
	}

	serviceItemID, err := r.lookupServiceItemID(req.Context(), serviceItemCode)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusBadRequest, "service_item_code not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to deactivate unit price")
		return
	}

	tierCode := strings.TrimSpace(req.URL.Query().Get("tier_code"))
	hasTier := tierCode != ""
	var tierID int64
	if hasTier {
		tierID, err = r.lookupTierID(req.Context(), tierCode)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "tier_code not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to deactivate unit price")
			return
		}
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	var result sql.Result
	if hasTier {
		result, err = r.db.ExecContext(req.Context(), `
			UPDATE unit_prices
			SET effective_to = ?
			WHERE service_item_id = ?
				AND tier_id = ?
				AND effective_to IS NULL;
		`, now, serviceItemID, tierID)
	} else {
		result, err = r.db.ExecContext(req.Context(), `
			UPDATE unit_prices
			SET effective_to = ?
			WHERE service_item_id = ?
				AND tier_id IS NULL
				AND effective_to IS NULL;
		`, now, serviceItemID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to deactivate unit price")
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to deactivate unit price")
		return
	}
	if affected == 0 {
		writeError(w, http.StatusNotFound, "active unit price not found")
		return
	}

	writeJSON(w, http.StatusOK, deactivateUnitPriceResponse{Deactivated: true})
}

func (r *routes) handlePublicTiers(w http.ResponseWriter, req *http.Request) {
	const query = `
		SELECT
			t.id,
			t.code,
			t.name,
			si.code,
			si.name,
			si.unit,
			tdi.included_units
		FROM tiers t
		LEFT JOIN tier_default_items tdi ON tdi.tier_id = t.id
		LEFT JOIN service_items si ON si.id = tdi.service_item_id
		ORDER BY t.id ASC, si.id ASC;
	`

	rows, err := r.db.QueryContext(req.Context(), query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tiers")
		return
	}
	defer rows.Close()

	tiers := make([]publicTierResponse, 0)
	tierIndex := make(map[int64]int)
	for rows.Next() {
		var (
			tierID        int64
			tierCode      string
			tierName      string
			itemCode      sql.NullString
			itemName      sql.NullString
			itemUnit      sql.NullString
			includedUnits sql.NullInt64
		)
		if err := rows.Scan(&tierID, &tierCode, &tierName, &itemCode, &itemName, &itemUnit, &includedUnits); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read tiers")
			return
		}

		idx, found := tierIndex[tierID]
		if !found {
			idx = len(tiers)
			tierIndex[tierID] = idx
			tiers = append(tiers, publicTierResponse{Code: tierCode, Name: tierName, DefaultItems: []publicTierItemResponse{}})
		}

		if itemCode.Valid {
			tiers[idx].DefaultItems = append(tiers[idx].DefaultItems, publicTierItemResponse{
				Code:          itemCode.String,
				Name:          itemName.String,
				Unit:          itemUnit.String,
				IncludedUnits: includedUnits.Int64,
			})
		}
	}

	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tiers")
		return
	}

	writeJSON(w, http.StatusOK, listPublicTiersResponse{Tiers: tiers})
}

func (r *routes) handlePublicEstimate(w http.ResponseWriter, req *http.Request) {
	var payload publicEstimateRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.TierCode = strings.TrimSpace(payload.TierCode)
	if payload.TierCode == "" {
		writeError(w, http.StatusBadRequest, "tier_code is required")
		return
	}

	var (
		tierID   int64
		tierName string
	)
	err := r.db.QueryRowContext(req.Context(), `SELECT id, name FROM tiers WHERE code = ?;`, payload.TierCode).Scan(&tierID, &tierName)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "tier not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load tier")
		return
	}

	const itemsQuery = `
		SELECT
			si.id,
			si.code,
			si.name,
			si.unit,
			tdi.included_units
		FROM tier_default_items tdi
		JOIN service_items si ON si.id = tdi.service_item_id
		WHERE tdi.tier_id = ?
		ORDER BY si.id ASC;
	`

	rows, err := r.db.QueryContext(req.Context(), itemsQuery, tierID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load tier items")
		return
	}
	defer rows.Close()

	response := publicEstimateResponse{
		TierCode: payload.TierCode,
		TierName: tierName,
		Items:    make([]publicEstimateItemResponse, 0),
	}

	for rows.Next() {
		var (
			serviceItemID int64
			item          publicEstimateItemResponse
		)
		if err := rows.Scan(&serviceItemID, &item.Code, &item.Name, &item.Unit, &item.IncludedUnits); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read tier items")
			return
		}

		price, found, err := r.lookupActiveUnitPrice(req.Context(), serviceItemID, tierID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load unit prices")
			return
		}

		if !found {
			item.MissingPrice = true
			response.Items = append(response.Items, item)
			continue
		}

		item.MissingPrice = false
		item.PricePerUnitMicros = price.PricePerUnitMicros
		item.LineTotalMicros = item.IncludedUnits * price.PricePerUnitMicros
		item.Currency = price.Currency

		if response.Currency == "" {
			response.Currency = price.Currency
		}
		response.TotalPriceMicros += item.LineTotalMicros
		response.Items = append(response.Items, item)
	}

	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load tier items")
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (r *routes) handleCreateSubscription(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var payload createSubscriptionRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.TierCode = strings.TrimSpace(payload.TierCode)
	if payload.TierCode == "" {
		writeError(w, http.StatusBadRequest, "tier_code is required")
		return
	}

	tierID, tierName, err := r.lookupTier(req.Context(), payload.TierCode)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "tier not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load tier")
		return
	}

	tx, err := r.db.BeginTx(req.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create subscription")
		return
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(req.Context(), `
		UPDATE subscriptions
		SET status = 'ended', ended_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND status = 'active' AND ended_at IS NULL;
	`, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to replace active subscription")
		return
	}

	result, err := tx.ExecContext(req.Context(), `
		INSERT INTO subscriptions(user_id, tier_id, status, started_at)
		VALUES (?, ?, 'active', CURRENT_TIMESTAMP);
	`, user.ID, tierID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create subscription")
		return
	}

	subscriptionID, err := result.LastInsertId()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create subscription")
		return
	}

	seenCodes := make(map[string]struct{})
	for _, override := range payload.Overrides {
		code := strings.TrimSpace(override.ServiceItemCode)
		if code == "" {
			writeError(w, http.StatusBadRequest, "override service_item_code is required")
			return
		}
		if override.IncludedUnits < 0 {
			writeError(w, http.StatusBadRequest, "override included_units must be non-negative")
			return
		}
		if _, exists := seenCodes[code]; exists {
			writeError(w, http.StatusBadRequest, "duplicate override service_item_code")
			return
		}
		seenCodes[code] = struct{}{}

		serviceItemID, err := lookupServiceItemID(req.Context(), tx, code)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "override service_item_code not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create subscription")
			return
		}

		if _, err := tx.ExecContext(req.Context(), `
			INSERT INTO subscription_overrides(subscription_id, service_item_id, included_units)
			VALUES (?, ?, ?);
		`, subscriptionID, serviceItemID, override.IncludedUnits); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create subscription")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create subscription")
		return
	}

	quotas, err := r.loadEffectiveQuotas(req.Context(), tierID, subscriptionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load subscription")
		return
	}

	writeJSON(w, http.StatusCreated, getSubscriptionResponse{Subscription: subscriptionResponse{
		TierCode: payload.TierCode,
		TierName: tierName,
		Quotas:   quotas,
	}})
}

func (r *routes) handleGetSubscription(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	subscription, found, err := r.loadActiveSubscription(req.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load subscription")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "active subscription not found")
		return
	}

	writeJSON(w, http.StatusOK, getSubscriptionResponse{Subscription: subscription})
}

func (r *routes) handleAIRequest(w http.ResponseWriter, req *http.Request) {
	rawAPIKey := strings.TrimSpace(req.Header.Get("X-API-Key"))
	if rawAPIKey == "" {
		writeError(w, http.StatusUnauthorized, "x-api-key header is required")
		return
	}

	authResult, ok, err := r.apiKey.AuthenticateKey(req.Context(), rawAPIKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to authenticate api key")
		return
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid api key")
		return
	}

	var payload aiRequestPayload
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	payload.ServiceItemCode = strings.TrimSpace(payload.ServiceItemCode)
	if payload.ServiceItemCode == "" {
		writeError(w, http.StatusBadRequest, "service_item_code is required")
		return
	}
	if payload.Quantity <= 0 {
		payload.Quantity = 1
	}

	subscriptionID, tierID, startedAt, found, err := r.lookupActiveSubscriptionContext(req.Context(), authResult.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load active subscription")
		return
	}
	if !found {
		writeError(w, http.StatusForbidden, "active subscription not found")
		return
	}

	serviceItemID, err := r.lookupServiceItemID(req.Context(), payload.ServiceItemCode)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusBadRequest, "service_item_code not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to process request")
		return
	}

	includedUnits, found, err := r.lookupEffectiveIncludedUnits(req.Context(), tierID, subscriptionID, serviceItemID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to process request")
		return
	}
	if !found {
		writeError(w, http.StatusForbidden, "quota not configured for service item")
		return
	}

	tx, err := r.db.BeginTx(req.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to process request")
		return
	}
	defer func() { _ = tx.Rollback() }()

	usedUnits, err := lookupUsedUnitsInSubscriptionWindow(req.Context(), tx, authResult.UserID, serviceItemID, startedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to process request")
		return
	}

	if usedUnits+payload.Quantity > includedUnits {
		remainingUnits := includedUnits - usedUnits
		if remainingUnits < 0 {
			remainingUnits = 0
		}
		writeJSON(w, http.StatusTooManyRequests, aiRequestResponse{
			Allowed:        false,
			IncludedUnits:  includedUnits,
			UsedUnits:      usedUnits,
			RemainingUnits: remainingUnits,
		})
		return
	}

	if _, err := tx.ExecContext(req.Context(), `
		INSERT INTO usage_records(user_id, api_key_id, service_item_id, quantity, usage_timestamp)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP);
	`, authResult.UserID, authResult.APIKeyID, serviceItemID, payload.Quantity); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record usage")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record usage")
		return
	}

	usedAfter := usedUnits + payload.Quantity
	remainingUnits := includedUnits - usedAfter
	if remainingUnits < 0 {
		remainingUnits = 0
	}

	writeJSON(w, http.StatusOK, aiRequestResponse{
		Allowed:        true,
		IncludedUnits:  includedUnits,
		UsedUnits:      usedAfter,
		RemainingUnits: remainingUnits,
	})
}

func (r *routes) loadActiveSubscription(ctx context.Context, userID int64) (subscriptionResponse, bool, error) {
	const query = `
		SELECT
			s.id,
			t.id,
			t.code,
			t.name
		FROM subscriptions s
		JOIN tiers t ON t.id = s.tier_id
		WHERE s.user_id = ?
			AND s.status = 'active'
			AND s.ended_at IS NULL
		ORDER BY s.started_at DESC, s.id DESC
		LIMIT 1;
	`

	var (
		subscriptionID int64
		tierID         int64
		response       subscriptionResponse
	)
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&subscriptionID, &tierID, &response.TierCode, &response.TierName)
	if errors.Is(err, sql.ErrNoRows) {
		return subscriptionResponse{}, false, nil
	}
	if err != nil {
		return subscriptionResponse{}, false, err
	}

	quotas, err := r.loadEffectiveQuotas(ctx, tierID, subscriptionID)
	if err != nil {
		return subscriptionResponse{}, false, err
	}
	response.Quotas = quotas

	return response, true, nil
}

func (r *routes) lookupActiveSubscriptionContext(ctx context.Context, userID int64) (int64, int64, string, bool, error) {
	const query = `
		SELECT
			s.id,
			s.tier_id,
			s.started_at
		FROM subscriptions s
		WHERE s.user_id = ?
			AND s.status = 'active'
			AND s.ended_at IS NULL
		ORDER BY s.started_at DESC, s.id DESC
		LIMIT 1;
	`

	var (
		subscriptionID int64
		tierID         int64
		startedAt      string
	)
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&subscriptionID, &tierID, &startedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, "", false, nil
	}
	if err != nil {
		return 0, 0, "", false, err
	}

	return subscriptionID, tierID, startedAt, true, nil
}

func (r *routes) lookupEffectiveIncludedUnits(ctx context.Context, tierID, subscriptionID, serviceItemID int64) (int64, bool, error) {
	const query = `
		SELECT COALESCE(so.included_units, tdi.included_units) AS included_units
		FROM service_items si
		LEFT JOIN tier_default_items tdi
			ON tdi.service_item_id = si.id
			AND tdi.tier_id = ?
		LEFT JOIN subscription_overrides so
			ON so.service_item_id = si.id
			AND so.subscription_id = ?
		WHERE si.id = ?
			AND (tdi.id IS NOT NULL OR so.id IS NOT NULL)
		LIMIT 1;
	`

	var includedUnits int64
	err := r.db.QueryRowContext(ctx, query, tierID, subscriptionID, serviceItemID).Scan(&includedUnits)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	return includedUnits, true, nil
}

func lookupUsedUnitsInSubscriptionWindow(ctx context.Context, tx *sql.Tx, userID, serviceItemID int64, startedAt string) (int64, error) {
	const query = `
		SELECT COALESCE(SUM(quantity), 0)
		FROM usage_records
		WHERE user_id = ?
			AND service_item_id = ?
			AND usage_timestamp >= ?;
	`

	var usedUnits int64
	err := tx.QueryRowContext(ctx, query, userID, serviceItemID, startedAt).Scan(&usedUnits)
	if err != nil {
		return 0, err
	}

	return usedUnits, nil
}

func (r *routes) loadEffectiveQuotas(ctx context.Context, tierID, subscriptionID int64) ([]subscriptionQuotaResponse, error) {
	const query = `
		SELECT
			si.code,
			si.name,
			si.unit,
			COALESCE(so.included_units, tdi.included_units) AS included_units
		FROM service_items si
		LEFT JOIN tier_default_items tdi
			ON tdi.service_item_id = si.id
			AND tdi.tier_id = ?
		LEFT JOIN subscription_overrides so
			ON so.service_item_id = si.id
			AND so.subscription_id = ?
		WHERE tdi.id IS NOT NULL OR so.id IS NOT NULL
		ORDER BY si.id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, tierID, subscriptionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	quotas := make([]subscriptionQuotaResponse, 0)
	for rows.Next() {
		var item subscriptionQuotaResponse
		if err := rows.Scan(&item.ServiceItemCode, &item.ServiceItemName, &item.Unit, &item.IncludedUnits); err != nil {
			return nil, err
		}
		quotas = append(quotas, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return quotas, nil
}

func (r *routes) lookupTier(ctx context.Context, tierCode string) (int64, string, error) {
	var (
		tierID   int64
		tierName string
	)
	err := r.db.QueryRowContext(ctx, `SELECT id, name FROM tiers WHERE code = ?;`, tierCode).Scan(&tierID, &tierName)
	if err != nil {
		return 0, "", err
	}
	return tierID, tierName, nil
}

func (r *routes) lookupTierID(ctx context.Context, tierCode string) (int64, error) {
	var tierID int64
	err := r.db.QueryRowContext(ctx, `SELECT id FROM tiers WHERE code = ?;`, tierCode).Scan(&tierID)
	if err != nil {
		return 0, err
	}
	return tierID, nil
}

func (r *routes) lookupServiceItemID(ctx context.Context, serviceItemCode string) (int64, error) {
	var serviceItemID int64
	err := r.db.QueryRowContext(ctx, `SELECT id FROM service_items WHERE code = ?;`, serviceItemCode).Scan(&serviceItemID)
	if err != nil {
		return 0, err
	}
	return serviceItemID, nil
}

func lookupServiceItemID(ctx context.Context, tx *sql.Tx, serviceItemCode string) (int64, error) {
	var serviceItemID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM service_items WHERE code = ?;`, serviceItemCode).Scan(&serviceItemID)
	if err != nil {
		return 0, err
	}
	return serviceItemID, nil
}

func validateAndNormalizeCurrency(raw string) (string, error) {
	if raw == "" {
		return "USD", nil
	}
	if len(raw) != 3 {
		return "", errors.New("currency must be 3-letter uppercase code")
	}
	for _, ch := range raw {
		if ch < 'A' || ch > 'Z' {
			return "", errors.New("currency must be 3-letter uppercase code")
		}
	}
	return raw, nil
}

type unitPriceRow struct {
	PricePerUnitMicros int64
	Currency           string
}

func (r *routes) lookupActiveUnitPrice(ctx context.Context, serviceItemID, tierID int64) (unitPriceRow, bool, error) {
	const query = `
		SELECT
			price_per_unit_micros,
			currency
		FROM unit_prices
		WHERE service_item_id = ?
			AND effective_to IS NULL
			AND (tier_id = ? OR tier_id IS NULL)
		ORDER BY
			CASE WHEN tier_id = ? THEN 0 ELSE 1 END,
			effective_from DESC
		LIMIT 1;
	`

	var result unitPriceRow
	err := r.db.QueryRowContext(ctx, query, serviceItemID, tierID, tierID).Scan(&result.PricePerUnitMicros, &result.Currency)
	if errors.Is(err, sql.ErrNoRows) {
		return unitPriceRow{}, false, nil
	}
	if err != nil {
		return unitPriceRow{}, false, err
	}

	return result, true, nil
}

func extractBearerToken(rawAuthHeader string) (string, error) {
	authHeader := strings.TrimSpace(rawAuthHeader)
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", errors.New("missing bearer token")
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
	if token == "" {
		return "", errors.New("missing bearer token")
	}

	return token, nil
}

func parsePathID(raw string) (int64, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 0, errors.New("id is required")
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func parseQueryLimit(raw string, fallback int) int {
	v := strings.TrimSpace(raw)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	if n > 100 {
		return 100
	}
	return n
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
