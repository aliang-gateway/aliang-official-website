package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"ai-api-portal/backend/internal/apikey"
	"ai-api-portal/backend/internal/article"
	"ai-api-portal/backend/internal/auth"
	"ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/fulfillment"
	"ai-api-portal/backend/internal/model"
	"ai-api-portal/backend/internal/proxy"
	portalstripe "ai-api-portal/backend/internal/stripe"
	"ai-api-portal/backend/internal/sub2apiauth"
	"ai-api-portal/backend/internal/user"
)

type routes struct {
	db                   *sql.DB
	sqlDialect           string
	apiKey               *apikey.Service
	articleSvc           *article.Service
	fulfillmentSvc       *fulfillment.Service
	userSvc              *user.Service
	sub2apiAuth          *sub2apiauth.Service
	proxyClient          *proxy.Client
	stripeClient         *portalstripe.Client
	adminBootstrapSecret string
}

type RoutesOptions struct {
	UserService          *user.Service
	ProxyClient          *proxy.Client
	StripeClient         *portalstripe.Client
	AdminBootstrapSecret string
	SQLDialect           string
}

var errForbiddenFilteredPayload = errors.New("filtered payload forbidden")

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
	Code string `json:"code"`
}

type adminPaymentSuccessRequest struct {
	PaymentEventID  string                          `json:"payment_event_id"`
	OrderID         string                          `json:"order_id"`
	Provider        string                          `json:"provider"`
	UserID          int64                           `json:"user_id"`
	SubscriptionID  *int64                          `json:"subscription_id"`
	BalanceRecharge *proxy.UpdateUserBalanceRequest `json:"balance_recharge,omitempty"`
	APIKey          *proxy.CreateUserAPIKeyRequest  `json:"api_key,omitempty"`
	TierCode        string                          `json:"tier_code,omitempty"`
	Payload         json.RawMessage                 `json:"payload,omitempty"`
}

type createPackageCheckoutSessionRequest struct {
	TierCode string `json:"tier_code"`
}

type createPackageCheckoutSessionResponse struct {
	SessionID   string `json:"session_id"`
	CheckoutURL string `json:"checkout_url"`
}

type checkoutPackageStatusResponse struct {
	Status            string                  `json:"status"`
	Provider          string                  `json:"provider"`
	CheckoutSessionID string                  `json:"checkout_session_id"`
	PaymentEventID    *string                 `json:"payment_event_id,omitempty"`
	TierCode          string                  `json:"tier_code"`
	PackageName       string                  `json:"package_name"`
	AmountMinor       int64                   `json:"amount_minor"`
	Currency          string                  `json:"currency"`
	FulfillmentJob    *fulfillmentJobResponse `json:"fulfillment_job,omitempty"`
}

type adminPaymentRecordResponse struct {
	ID                int64                   `json:"id"`
	Provider          string                  `json:"provider"`
	CheckoutSessionID string                  `json:"checkout_session_id"`
	PaymentEventID    *string                 `json:"payment_event_id,omitempty"`
	UserID            int64                   `json:"user_id"`
	TierCode          string                  `json:"tier_code"`
	PackageName       string                  `json:"package_name"`
	AmountMinor       int64                   `json:"amount_minor"`
	Currency          string                  `json:"currency"`
	Status            string                  `json:"status"`
	OrderStatus       string                  `json:"order_status"`
	Replayable        bool                    `json:"replayable"`
	FulfillmentJob    *fulfillmentJobResponse `json:"fulfillment_job,omitempty"`
}

type listAdminPaymentRecordsResponse struct {
	Records []adminPaymentRecordResponse `json:"records"`
}

type fulfillmentJobResponse struct {
	ID             int64   `json:"id"`
	EventType      string  `json:"event_type"`
	Status         string  `json:"status"`
	UserID         *int64  `json:"user_id,omitempty"`
	SubscriptionID *int64  `json:"subscription_id,omitempty"`
	ErrorMessage   *string `json:"error_message,omitempty"`
	RetryCount     int     `json:"retry_count"`
	IdempotencyKey *string `json:"idempotency_key,omitempty"`
}

type adminPaymentSuccessResponse struct {
	Job fulfillmentJobResponse `json:"job"`
}

type adminGroupResponse struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Platform         string `json:"platform,omitempty"`
	Type             string `json:"type,omitempty"`
	SubscriptionType string `json:"subscription_type,omitempty"`
}

type adminAvailableGroupsResponse struct {
	Groups []adminGroupResponse `json:"groups"`
}

type adminPackageRequest struct {
	Code         string  `json:"code,omitempty"`
	Name         string  `json:"name"`
	GroupIDs     []int64 `json:"group_ids"`
	PriceMicros  int64   `json:"price_micros"`
	ValueType    string  `json:"value_type"`
	ValueAmount  int64   `json:"value_amount"`
	Description  string  `json:"description"`
	FeaturesJSON string  `json:"features_json"`
	IsEnabled    *bool   `json:"is_enabled,omitempty"`
}

type adminPackageResponse struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	GroupIDs    []int64  `json:"group_ids"`
	PriceMicros int64    `json:"price_micros"`
	ValueType   string   `json:"value_type"`
	ValueAmount int64    `json:"value_amount"`
	Description string   `json:"description"`
	Features    []string `json:"features"`
	IsEnabled   bool     `json:"is_enabled"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type listAdminPackagesResponse struct {
	Packages []adminPackageResponse `json:"packages"`
}

type publicPackageResponse struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	PriceMicros int64    `json:"price_micros"`
	ValueType   string   `json:"value_type"`
	ValueAmount int64    `json:"value_amount"`
	Description string   `json:"description"`
	Features    []string `json:"features"`
}

type listPublicPackagesResponse struct {
	Packages []publicPackageResponse `json:"packages"`
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

type publicArticleDTO struct {
	Slug            string  `json:"slug"`
	Title           string  `json:"title"`
	Excerpt         *string `json:"excerpt,omitempty"`
	CoverImageURL   *string `json:"cover_image_url,omitempty"`
	Tag             *string `json:"tag,omitempty"`
	ReadTime        *string `json:"read_time,omitempty"`
	AuthorName      *string `json:"author_name,omitempty"`
	AuthorAvatarURL *string `json:"author_avatar_url,omitempty"`
	PublishedAt     string  `json:"published_at"`
}

type publicArticleListResponse struct {
	Articles []publicArticleDTO `json:"articles"`
}

type publicArticleDetailDTO struct {
	Slug            string  `json:"slug"`
	Title           string  `json:"title"`
	Excerpt         *string `json:"excerpt,omitempty"`
	CoverImageURL   *string `json:"cover_image_url,omitempty"`
	Tag             *string `json:"tag,omitempty"`
	ReadTime        *string `json:"read_time,omitempty"`
	AuthorName      *string `json:"author_name,omitempty"`
	AuthorAvatarURL *string `json:"author_avatar_url,omitempty"`
	PublishedAt     string  `json:"published_at"`
	MDXBody         string  `json:"mdx_body"`
}

type publicArticleDetailResponse struct {
	Article publicArticleDetailDTO `json:"article"`
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

type adminCreateArticleRequest struct {
	Slug            string  `json:"slug"`
	Title           string  `json:"title"`
	Excerpt         *string `json:"excerpt"`
	CoverImageURL   *string `json:"cover_image_url"`
	Tag             *string `json:"tag"`
	ReadTime        *string `json:"read_time"`
	AuthorName      *string `json:"author_name"`
	AuthorAvatarURL *string `json:"author_avatar_url"`
	AuthorIcon      *string `json:"author_icon"`
	MDXBody         string  `json:"mdx_body"`
	Status          string  `json:"status"`
}

type adminUpdateArticleRequest struct {
	LegacyID        *int64  `json:"legacy_id"`
	Slug            *string `json:"slug"`
	Title           *string `json:"title"`
	Excerpt         *string `json:"excerpt"`
	CoverImageURL   *string `json:"cover_image_url"`
	Tag             *string `json:"tag"`
	ReadTime        *string `json:"read_time"`
	AuthorName      *string `json:"author_name"`
	AuthorAvatarURL *string `json:"author_avatar_url"`
	AuthorIcon      *string `json:"author_icon"`
	MDXBody         *string `json:"mdx_body"`
	Status          *string `json:"status"`
}

type adminArticleDTO struct {
	ID              int64   `json:"id"`
	LegacyID        *int64  `json:"legacy_id"`
	Slug            string  `json:"slug"`
	Title           string  `json:"title"`
	Excerpt         *string `json:"excerpt"`
	CoverImageURL   *string `json:"cover_image_url"`
	Tag             *string `json:"tag"`
	ReadTime        *string `json:"read_time"`
	AuthorName      *string `json:"author_name"`
	AuthorAvatarURL *string `json:"author_avatar_url"`
	AuthorIcon      *string `json:"author_icon"`
	MDXBody         string  `json:"mdx_body"`
	Status          string  `json:"status"`
	PublishedAt     *string `json:"published_at"`
	CreatedByUserID *int64  `json:"created_by_user_id"`
	UpdatedByUserID *int64  `json:"updated_by_user_id"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type adminArticleListResponse struct {
	Articles []adminArticleDTO `json:"articles"`
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

const (
	adminArticleStatusDraft     = "draft"
	adminArticleStatusPublished = "published"
	maxArticleSlugLength        = 128
)

var articleSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

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
		sqlDialect:           strings.TrimSpace(opts.SQLDialect),
		apiKey:               apikey.NewService(database),
		articleSvc:           article.NewService(database),
		fulfillmentSvc:       fulfillment.NewServiceWithDialect(database, strings.TrimSpace(opts.SQLDialect)),
		userSvc:              userSvc,
		sub2apiAuth:          sub2apiauth.NewServiceWithDialect(database, strings.TrimSpace(opts.SQLDialect)),
		proxyClient:          opts.ProxyClient,
		stripeClient:         opts.StripeClient,
		adminBootstrapSecret: strings.TrimSpace(opts.AdminBootstrapSecret),
	}
	authenticated := auth.RequireUserWithDialect(database, r.sqlDialect)

	mux.HandleFunc("POST /users", r.handleCreateUser)
	mux.HandleFunc("POST /auth/register", r.handleAuthRegisterPassthrough)
	mux.HandleFunc("POST /auth/login", r.handleAuthLoginPassthrough)
	mux.HandleFunc("GET /auth/me", r.handleAuthMePassthrough)
	mux.HandleFunc("POST /auth/refresh", r.handleAuthRefreshPassthrough)
	mux.HandleFunc("POST /auth/logout", r.handleAuthLogoutPassthrough)
	if r.proxyClient != nil {
		mux.HandleFunc("GET /dashboard/home", r.handleDashboardHomePassthrough)
		mux.HandleFunc("GET /dashboard/details", r.handleDashboardDetailsPassthrough)
		mux.HandleFunc("GET /dashboard/trend", r.handleDashboardDetailsPassthrough)
		mux.HandleFunc("GET /dashboard/models", r.handleDashboardModelsPassthrough)
		mux.HandleFunc("GET /dashboard/usage", r.handleDashboardUsagePassthrough)
		mux.HandleFunc("GET /subscription", r.handleSubscriptionProgressPassthrough)
		mux.HandleFunc("GET /dashboard/account", r.handleDashboardAccountPassthrough)

		// Sub2API passthrough: API keys
		mux.Handle("GET /api-keys", authenticated(http.HandlerFunc(r.handleAPIKeysListPassthrough)))
		mux.Handle("POST /api-keys", authenticated(http.HandlerFunc(r.handleAPIKeysCreatePassthrough)))
		mux.Handle("GET /api-keys/{id}", authenticated(http.HandlerFunc(r.handleAPIKeyDetailPassthrough)))
		mux.Handle("PUT /api-keys/{id}", authenticated(http.HandlerFunc(r.handleAPIKeyUpdatePassthrough)))
		mux.Handle("DELETE /api-keys/{id}", authenticated(http.HandlerFunc(r.handleAPIKeyDeletePassthrough)))

		// Sub2API passthrough: groups
		mux.Handle("GET /groups/available", authenticated(http.HandlerFunc(r.handleGroupsAvailablePassthrough)))

		// Sub2API passthrough: als_subscriptions
		mux.HandleFunc("GET /subscriptions/summary", r.handleSubscriptionsSummaryPassthrough)
		mux.HandleFunc("GET /subscriptions/active", r.handleSubscriptionsActivePassthrough)
		mux.HandleFunc("GET /subscriptions/all", r.handleSubscriptionsAllPassthrough)

		// Sub2API passthrough: redeem & usage
		mux.HandleFunc("GET /redeem/history", r.handleRedeemHistoryPassthrough)
		mux.HandleFunc("GET /usage/stats", r.handleUsageStatsPassthrough)
	} else {
		mux.Handle("GET /subscription", authenticated(http.HandlerFunc(r.handleGetSubscription)))
		mux.Handle("POST /api-keys", authenticated(http.HandlerFunc(r.handleCreateAPIKey)))
		mux.Handle("DELETE /api-keys/{id}", authenticated(http.HandlerFunc(r.handleRevokeAPIKey)))
	}
	mux.HandleFunc("POST /auth/verify-email", r.handleVerifyEmail)
	mux.HandleFunc("POST /auth/forgot-password", r.handleForgotPassword)
	mux.HandleFunc("POST /auth/reset-password", r.handleResetPassword)
	mux.HandleFunc("POST /webhooks/stripe", r.handleStripeWebhook)
	mux.Handle("GET /user/me", authenticated(http.HandlerFunc(r.handleGetMe)))
	mux.Handle("PUT /user/me", authenticated(http.HandlerFunc(r.handleUpdateMe)))
	mux.Handle("PUT /user/password", authenticated(http.HandlerFunc(r.handleChangePassword)))
	mux.Handle("POST /user/password", authenticated(http.HandlerFunc(r.handleSetInitialPassword)))
	mux.Handle("GET /wallet", authenticated(http.HandlerFunc(r.handleGetWallet)))
	mux.Handle("GET /wallet/transactions", authenticated(http.HandlerFunc(r.handleListWalletTransactions)))
	mux.Handle("POST /wallet/redeem", authenticated(http.HandlerFunc(r.handleRedeemCard)))
	mux.Handle("GET /checkout/package/status", authenticated(http.HandlerFunc(r.handleGetPackageCheckoutStatus)))
	mux.Handle("GET /fulfillment/jobs/{id}", authenticated(http.HandlerFunc(r.handleGetFulfillmentJob)))
	mux.Handle("POST /profiles", authenticated(http.HandlerFunc(r.handleCreateProfileConfig)))
	mux.Handle("GET /profiles", authenticated(http.HandlerFunc(r.handleListProfileConfigs)))
	mux.Handle("GET /profiles/{id}", authenticated(http.HandlerFunc(r.handleGetProfileConfig)))
	mux.Handle("PUT /profiles/{id}", authenticated(http.HandlerFunc(r.handleUpdateProfileConfig)))
	mux.Handle("DELETE /profiles/{id}", authenticated(http.HandlerFunc(r.handleDeleteProfileConfig)))
	mux.Handle("DELETE /session", authenticated(http.HandlerFunc(r.handleLogout)))
	mux.Handle("GET /sessions", authenticated(http.HandlerFunc(r.handleListSessions)))
	mux.HandleFunc("GET /public/tiers", r.handlePublicTiers)
	mux.HandleFunc("POST /public/estimate", r.handlePublicEstimate)
	mux.HandleFunc("GET /public/articles", r.handlePublicListArticles)
	mux.HandleFunc("GET /public/articles/{slug}", r.handlePublicGetArticle)
	mux.HandleFunc("GET /public/packages", http.HandlerFunc(r.handlePublicListPackages))
	mux.Handle("POST /subscription", authenticated(http.HandlerFunc(r.handleCreateSubscription)))
	mux.Handle("POST /checkout/package", authenticated(http.HandlerFunc(r.handleCreatePackageCheckoutSession)))
	mux.Handle("GET /admin/unit-prices", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListUnitPrices))))
	mux.Handle("PUT /admin/unit-prices", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminSetUnitPrice))))
	mux.Handle("DELETE /admin/unit-prices", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminDeactivateUnitPrice))))
	mux.Handle("POST /admin/fulfillment/payment-success", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminPaymentSuccess))))
	mux.Handle("GET /admin/fulfillment/jobs/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminGetFulfillmentJob))))
	mux.Handle("POST /admin/fulfillment/jobs/{id}/replay", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminReplayFulfillmentJob))))
	mux.Handle("GET /admin/groups/available", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListAvailableGroups))))
	mux.Handle("GET /admin/packages", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListPackages))))
	mux.Handle("POST /admin/packages", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminCreatePackage))))
	mux.Handle("GET /admin/packages/{code}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminGetPackage))))
	mux.Handle("PUT /admin/packages/{code}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminUpdatePackage))))
	mux.Handle("GET /admin/payments", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListPaymentRecords))))
	mux.Handle("GET /admin/articles", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListArticles))))
	mux.Handle("POST /admin/articles", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminCreateArticle))))
	mux.Handle("GET /admin/articles/{slug}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminGetArticle))))
	mux.Handle("PUT /admin/articles/{slug}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminUpdateArticle))))
	mux.Handle("DELETE /admin/articles/{slug}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminDeleteArticle))))
	mux.Handle("POST /admin/articles/{slug}/publish", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminPublishArticle))))
	mux.Handle("POST /admin/articles/{slug}/unpublish", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminUnpublishArticle))))
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

	const query = `INSERT INTO als_users(email, name, role) VALUES (?, ?, ?)`
	id, err := db.InsertID(req.Context(), r.sqlDialect, tx, query, "id", payload.Email, payload.Name, payload.Role)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to create user: %v", err))
		return
	}

	if _, err := tx.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `
		INSERT INTO als_sessions(user_id, token_hash, expires_at)
		VALUES (?, ?, ?);
	`), id, sessionTokenHash, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user session")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	writeJSON(w, http.StatusCreated, createUserResponse{ID: id, Email: payload.Email, Name: payload.Name, Role: payload.Role, SessionToken: plaintextSessionToken})
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

func (r *routes) handleAuthRegisterPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleAuthPassthrough(w, req, "/api/v1/auth/register")
}

func (r *routes) handleAuthLoginPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleAuthPassthrough(w, req, "/api/v1/auth/login")
}

func (r *routes) handleAuthMePassthrough(w http.ResponseWriter, req *http.Request) {
	if profile, handled, err := r.resolveLocalAuthMeProfile(req.Context(), req.Header.Get("Authorization")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve local session")
		return
	} else if handled {
		writeJSON(w, http.StatusOK, profile)
		return
	}

	r.handleAuthPassthrough(w, req, "/api/v1/auth/me")
}

func (r *routes) handleAuthRefreshPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleAuthPassthrough(w, req, "/api/v1/auth/refresh")
}

func (r *routes) handleAuthLogoutPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleAuthPassthrough(w, req, "/api/v1/auth/logout")
}

func (r *routes) handleDashboardHomePassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/usage/dashboard/stats")
}

func (r *routes) handleDashboardDetailsPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/usage/dashboard/trend")
}

func (r *routes) handleDashboardModelsPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/usage/dashboard/models")
}

func (r *routes) handleDashboardUsagePassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/usage")
}

func (r *routes) resolveLocalAuthMeProfile(ctx context.Context, authHeader string) (*user.UserProfile, bool, error) {
	localSessionToken, err := extractBearerToken(authHeader)
	if err != nil {
		return nil, false, nil
	}

	userID, found, err := r.findLocalUserIDBySessionToken(ctx, localSessionToken)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}

	if r.proxyClient != nil && r.sub2apiAuth != nil {
		if _, err := r.sub2apiAuth.GetBearerTokenByUserID(ctx, userID); err == nil {
			return nil, false, nil
		} else if !errors.Is(err, sub2apiauth.ErrTokenNotFound) {
			return nil, false, err
		}
	}

	profile, err := r.userSvc.GetProfile(ctx, userID)
	if err != nil {
		return nil, false, err
	}

	return profile, true, nil
}

func (r *routes) handleSubscriptionProgressPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/subscriptions/progress")
}

func (r *routes) handleDashboardAccountPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/user/profile")
}

// ----- Sub2API passthrough handlers for API keys, groups, als_subscriptions, redeem, usage -----

func (r *routes) handleAPIKeysListPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleFilteredAPIKeysListPassthrough(w, req)
}

func (r *routes) handleAPIKeysCreatePassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/keys")
}

func (r *routes) handleAPIKeyDetailPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleFilteredAPIKeyDetailPassthrough(w, req)
}

func (r *routes) handleAPIKeyUpdatePassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleFilteredAPIKeyDetailPassthrough(w, req)
}

func (r *routes) handleAPIKeyDeletePassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleFilteredAPIKeyDetailPassthrough(w, req)
}

func (r *routes) handleGroupsAvailablePassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleFilteredGroupsAvailablePassthrough(w, req)
}

func (r *routes) handleFilteredGroupsAvailablePassthrough(w http.ResponseWriter, req *http.Request) {
	if r.proxyClient == nil {
		writeError(w, http.StatusInternalServerError, "proxy client is not configured")
		return
	}

	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if user.Role == "admin" {
		r.handleDashboardPassthrough(w, req, "/api/v1/groups/available")
		return
	}

	authorizedGroupIDs, err := r.loadAuthorizedGroupIDSet(req.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load authorized groups")
		return
	}
	if len(authorizedGroupIDs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"data": []map[string]any{}})
		return
	}

	filteredPayload, statusCode, headers, handled, err := r.filteredProxyJSONResponse(w, req, "/api/v1/groups/available", func(payload any) (any, error) {
		return filterGroupListPayloadByID(payload, authorizedGroupIDs)
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch groups")
		return
	}
	if !handled {
		return
	}
	writeForwardedJSON(w, statusCode, headers, filteredPayload)
}

func (r *routes) handleFilteredAPIKeysListPassthrough(w http.ResponseWriter, req *http.Request) {
	if r.proxyClient == nil {
		writeError(w, http.StatusInternalServerError, "proxy client is not configured")
		return
	}

	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if user.Role == "admin" {
		r.handleDashboardPassthrough(w, req, "/api/v1/keys")
		return
	}

	authorizedGroupIDs, err := r.loadAuthorizedGroupIDSet(req.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load authorized groups")
		return
	}
	if len(authorizedGroupIDs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"data": []map[string]any{}, "total": 0, "page": 1, "per_page": 20}})
		return
	}

	authorizedGroupIDs, err = r.loadAuthorizedVisibleGroupIDs(req, authorizedGroupIDs)
	if err != nil {
		if errors.Is(err, sub2apiauth.ErrTokenNotFound) {
			writeError(w, http.StatusUnauthorized, "upstream session unavailable")
			return
		}
		writeError(w, http.StatusBadGateway, "failed to load authorized group ids")
		return
	}
	if len(authorizedGroupIDs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"data": []map[string]any{}, "total": 0, "page": 1, "per_page": 20}})
		return
	}

	filteredPayload, statusCode, headers, handled, err := r.filteredProxyJSONResponse(w, req, "/api/v1/keys", func(payload any) (any, error) {
		return filterAPIKeyListPayload(payload, authorizedGroupIDs)
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch api keys")
		return
	}
	if !handled {
		return
	}
	writeForwardedJSON(w, statusCode, headers, filteredPayload)
}

func (r *routes) handleFilteredAPIKeyDetailPassthrough(w http.ResponseWriter, req *http.Request) {
	if r.proxyClient == nil {
		writeError(w, http.StatusInternalServerError, "proxy client is not configured")
		return
	}

	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if user.Role == "admin" {
		r.handleDashboardPassthrough(w, req, "/api/v1/keys/"+req.PathValue("id"))
		return
	}

	authorizedGroupIDs, err := r.loadAuthorizedGroupIDSet(req.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load authorized groups")
		return
	}
	if len(authorizedGroupIDs) == 0 {
		writeError(w, http.StatusForbidden, "group access forbidden")
		return
	}
	authorizedGroupIDs, err = r.loadAuthorizedVisibleGroupIDs(req, authorizedGroupIDs)
	if err != nil {
		if errors.Is(err, sub2apiauth.ErrTokenNotFound) {
			writeError(w, http.StatusUnauthorized, "upstream session unavailable")
			return
		}
		writeError(w, http.StatusBadGateway, "failed to load authorized group ids")
		return
	}
	if len(authorizedGroupIDs) == 0 {
		writeError(w, http.StatusForbidden, "group access forbidden")
		return
	}

	upstreamPath := "/api/v1/keys/" + req.PathValue("id")
	filteredPayload, statusCode, headers, handled, err := r.filteredProxyJSONResponse(w, req, upstreamPath, func(payload any) (any, error) {
		allowed, filterErr := isAPIKeyPayloadAuthorized(payload, authorizedGroupIDs)
		if filterErr != nil {
			return nil, filterErr
		}
		if !allowed {
			return nil, errForbiddenFilteredPayload
		}
		return payload, nil
	})
	if errors.Is(err, errForbiddenFilteredPayload) {
		writeError(w, http.StatusForbidden, "group access forbidden")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch api key")
		return
	}
	if !handled {
		return
	}
	writeForwardedJSON(w, statusCode, headers, filteredPayload)
}

func (r *routes) handleAdminListAvailableGroups(w http.ResponseWriter, req *http.Request) {
	if r.proxyClient == nil {
		writeError(w, http.StatusInternalServerError, "proxy client is not configured")
		return
	}

	resp, err := r.proxyClient.ListAdminGroups(req.Context(), req.URL.Query().Get("platform"))
	if err != nil {
		var apiErr *proxy.APIError
		if errors.As(err, &apiErr) {
			writeError(w, apiErr.StatusCode, apiErr.Message)
			return
		}
		writeError(w, http.StatusBadGateway, "failed to fetch admin groups")
		return
	}

	groups := make([]adminGroupResponse, 0, len(resp.Data))
	for _, group := range resp.Data {
		subscriptionType := strings.TrimSpace(group.SubscriptionType)
		if subscriptionType == "" {
			subscriptionType = strings.TrimSpace(group.Type)
		}
		groups = append(groups, adminGroupResponse{
			ID:               group.ID,
			Name:             group.Name,
			Platform:         group.Platform,
			Type:             subscriptionType,
			SubscriptionType: subscriptionType,
		})
	}

	writeJSON(w, http.StatusOK, adminAvailableGroupsResponse{Groups: groups})
}

func (r *routes) handleAdminListPackages(w http.ResponseWriter, req *http.Request) {
	packages, err := r.listAdminPackages(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list packages")
		return
	}
	writeJSON(w, http.StatusOK, listAdminPackagesResponse{Packages: packages})
}

func (r *routes) handleAdminListPaymentRecords(w http.ResponseWriter, req *http.Request) {
	records, err := r.listAdminPaymentRecords(req.Context(), parseQueryLimit(req.URL.Query().Get("limit"), 50))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list payment records")
		return
	}
	writeJSON(w, http.StatusOK, listAdminPaymentRecordsResponse{Records: records})
}

func (r *routes) handleAdminGetPackage(w http.ResponseWriter, req *http.Request) {
	packageCode := strings.TrimSpace(req.PathValue("code"))
	if packageCode == "" {
		writeError(w, http.StatusBadRequest, "package code is required")
		return
	}

	pkg, err := r.loadAdminPackageByCode(req.Context(), packageCode)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "package not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load package")
		return
	}

	writeJSON(w, http.StatusOK, pkg)
}

func (r *routes) handleAdminCreatePackage(w http.ResponseWriter, req *http.Request) {
	var payload adminPackageRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	normalized, err := normalizeAdminPackageRequest(payload, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	isEnabled := true
	if normalized.IsEnabled != nil {
		isEnabled = *normalized.IsEnabled
	}

	tx, err := r.db.BeginTx(req.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create package")
		return
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	tierID, err := db.InsertID(req.Context(), r.sqlDialect, tx, `
		INSERT INTO als_tiers(code, name, price_micros, value_type, value_amount, description, features_json, is_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "id", normalized.Code, normalized.Name, normalized.PriceMicros, normalized.ValueType, normalized.ValueAmount, normalized.Description, normalized.FeaturesJSON, isEnabled, now, now)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to create package: %v", err))
		return
	}

	if err := r.replaceTierGroupBindingsTx(req.Context(), tx, tierID, normalized.GroupIDs, now); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save package groups")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create package")
		return
	}

	pkg, err := r.loadAdminPackageByCode(req.Context(), normalized.Code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load package")
		return
	}

	writeJSON(w, http.StatusCreated, pkg)
}

func (r *routes) handleAdminUpdatePackage(w http.ResponseWriter, req *http.Request) {
	packageCode := strings.TrimSpace(req.PathValue("code"))
	if packageCode == "" {
		writeError(w, http.StatusBadRequest, "package code is required")
		return
	}

	var payload adminPackageRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	payload.Code = packageCode

	normalized, err := normalizeAdminPackageRequest(payload, false)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tierID, err := r.lookupTierID(req.Context(), packageCode)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "package not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update package")
		return
	}

	tx, err := r.db.BeginTx(req.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update package")
		return
	}
	defer func() { _ = tx.Rollback() }()

	isEnabledVal := false
	if normalized.IsEnabled != nil {
		isEnabledVal = *normalized.IsEnabled
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `UPDATE als_tiers SET name = ?, price_micros = ?, value_type = ?, value_amount = ?, description = ?, features_json = ?, is_enabled = ?, updated_at = ? WHERE id = ?;`), normalized.Name, normalized.PriceMicros, normalized.ValueType, normalized.ValueAmount, normalized.Description, normalized.FeaturesJSON, isEnabledVal, now, tierID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update package")
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update package")
		return
	}
	if affected == 0 {
		writeError(w, http.StatusNotFound, "package not found")
		return
	}

	if err := r.replaceTierGroupBindingsTx(req.Context(), tx, tierID, normalized.GroupIDs, now); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save package groups")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update package")
		return
	}

	pkg, err := r.loadAdminPackageByCode(req.Context(), packageCode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load package")
		return
	}

	writeJSON(w, http.StatusOK, pkg)
}

func (r *routes) handleSubscriptionsSummaryPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/subscriptions/summary")
}

func (r *routes) handleSubscriptionsActivePassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/subscriptions/active")
}

func (r *routes) handleSubscriptionsAllPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/subscriptions")
}

func (r *routes) handleRedeemHistoryPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/redeem/history")
}

func (r *routes) handleUsageStatsPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/usage/stats")
}

func (r *routes) handleAuthPassthrough(w http.ResponseWriter, req *http.Request, upstreamPath string) {
	if r.proxyClient == nil {
		writeError(w, http.StatusInternalServerError, "auth proxy is not configured")
		return
	}

	requestBody := []byte(nil)
	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to proxy auth request")
			return
		}
		_ = req.Body.Close()
		requestBody = bodyBytes
		req.Body = io.NopCloser(bytes.NewReader(requestBody))
	}

	requestEmail := ""
	requestRefreshToken := ""
	if upstreamPath == "/api/v1/auth/login" && len(requestBody) > 0 {
		requestEmail = extractAuthEmailFromRequestBody(requestBody)
	}
	if upstreamPath == "/api/v1/auth/refresh" && len(requestBody) > 0 {
		requestRefreshToken = extractAuthRefreshTokenFromRequestBody(requestBody)
	}

	forwarded := req.Clone(req.Context())
	forwarded.Header = cloneHeaders(req.Header)
	forwarded.Header.Del("X-User-Id")
	if err := r.replaceAuthorizationWithStoredUpstreamToken(req.Context(), forwarded.Header); err != nil {
		if errors.Is(err, sub2apiauth.ErrTokenNotFound) {
			writeError(w, http.StatusUnauthorized, "upstream session unavailable")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve upstream session")
		return
	}
	if requestBody != nil {
		forwarded.Body = io.NopCloser(bytes.NewReader(requestBody))
		forwarded.ContentLength = int64(len(requestBody))
	}

	resp, err := r.proxyClient.Do(req.Context(), forwarded, upstreamPath)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to proxy auth request")
		return
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		_ = resp.Body.Close()
		writeError(w, http.StatusBadGateway, "failed to proxy auth response")
		return
	}
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(responseBody))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 && (upstreamPath == "/api/v1/auth/login" || upstreamPath == "/api/v1/auth/refresh") {
		localUserID, found, err := r.captureSub2APITokens(req.Context(), req, requestEmail, requestRefreshToken, responseBody)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to persist auth session")
			return
		}
		if upstreamPath == "/api/v1/auth/login" && found {
			sessionToken, sessionErr := r.createLocalSessionToken(req.Context(), localUserID)
			if sessionErr != nil {
				writeError(w, http.StatusInternalServerError, "failed to create local session")
				return
			}
			responseBody, err = injectSessionTokenIntoAuthResponse(responseBody, sessionToken)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to finalize login response")
				return
			}
			resp.Body = io.NopCloser(bytes.NewReader(responseBody))
			resp.ContentLength = int64(len(responseBody))
			resp.Header.Set("Content-Length", strconv.Itoa(len(responseBody)))
		}
	}

	if err := proxy.CopyResponse(w, resp); err != nil {
		log.Printf("proxy auth response copy failed for %s: %v", upstreamPath, err)
		return
	}
}

func (r *routes) captureSub2APITokens(ctx context.Context, req *http.Request, requestEmail, requestRefreshToken string, responseBody []byte) (int64, bool, error) {
	accessToken, refreshToken, _, upstreamUserID, ok := extractSub2APITokensFromAuthResponse(responseBody)
	if !ok {
		return 0, false, nil
	}

	authIdentity := extractAuthIdentityFromResponse(responseBody)
	localUserID, found, err := r.resolveLocalUserIDForAuthTokens(ctx, authIdentity.Email, requestEmail, req.Header.Get("Authorization"), requestRefreshToken, refreshToken)
	if err != nil {
		return 0, false, err
	}
	if !found {
		localUserID, found, err = r.ensureLocalUser(ctx, authIdentity)
		if err != nil {
			return 0, false, err
		}
		if !found {
			return 0, false, nil
		}
	}

	if err := r.sub2apiAuth.UpsertToken(ctx, sub2apiauth.UpsertTokenInput{
		UserID:         localUserID,
		UpstreamUserID: upstreamUserID,
		AccessToken:    accessToken,
		RefreshToken:   refreshToken,
	}); err != nil {
		return 0, false, err
	}

	return localUserID, true, nil
}

func (r *routes) resolveLocalUserIDForAuthTokens(ctx context.Context, responseEmail, requestEmail, authHeader, requestRefreshToken string, refreshToken *string) (int64, bool, error) {
	for _, email := range []string{responseEmail, requestEmail} {
		if strings.TrimSpace(email) == "" {
			continue
		}
		userID, found, err := r.findLocalUserIDByEmail(ctx, email)
		if err != nil {
			return 0, false, err
		}
		if found {
			return userID, true, nil
		}
	}

	bearerToken, err := extractBearerToken(authHeader)
	if err == nil {
		userID, found, lookupErr := r.findLocalUserIDByStoredAccessToken(ctx, bearerToken)
		if lookupErr != nil {
			return 0, false, lookupErr
		}
		if found {
			return userID, true, nil
		}
	}

	if strings.TrimSpace(requestRefreshToken) != "" {
		userID, found, lookupErr := r.findLocalUserIDByStoredRefreshToken(ctx, requestRefreshToken)
		if lookupErr != nil {
			return 0, false, lookupErr
		}
		if found {
			return userID, true, nil
		}
	}

	if refreshToken != nil && strings.TrimSpace(*refreshToken) != "" {
		userID, found, lookupErr := r.findLocalUserIDByStoredRefreshToken(ctx, *refreshToken)
		if lookupErr != nil {
			return 0, false, lookupErr
		}
		if found {
			return userID, true, nil
		}
	}

	return 0, false, nil
}

func (r *routes) findLocalUserIDByEmail(ctx context.Context, email string) (int64, bool, error) {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return 0, false, nil
	}

	var userID int64
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT id
		FROM als_users
		WHERE LOWER(email) = LOWER(?)
		LIMIT 1;
	`), trimmed).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("query local user by email: %w", err)
	}

	return userID, true, nil
}

func (r *routes) findLocalUserIDByStoredAccessToken(ctx context.Context, accessToken string) (int64, bool, error) {
	trimmed := strings.TrimSpace(accessToken)
	if trimmed == "" {
		return 0, false, nil
	}

	var userID int64
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT user_id
		FROM als_sub2api_auth_tokens
		WHERE access_token = ?
		LIMIT 1;
	`), trimmed).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("query local user by stored access token: %w", err)
	}

	return userID, true, nil
}

func (r *routes) findLocalUserIDByStoredRefreshToken(ctx context.Context, refreshToken string) (int64, bool, error) {
	trimmed := strings.TrimSpace(refreshToken)
	if trimmed == "" {
		return 0, false, nil
	}

	var userID int64
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT user_id
		FROM als_sub2api_auth_tokens
		WHERE refresh_token = ?
		LIMIT 1;
	`), trimmed).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("query local user by stored refresh token: %w", err)
	}

	return userID, true, nil
}

func (r *routes) findLocalUserRoleByID(ctx context.Context, userID int64) (string, bool, error) {
	if userID <= 0 {
		return "", false, nil
	}

	var role string
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT role
		FROM als_users
		WHERE id = ?
		LIMIT 1;
	`), userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("query local user role: %w", err)
	}

	return strings.TrimSpace(role), true, nil
}

func extractAuthEmailFromRequestBody(body []byte) string {
	type loginPayload struct {
		Email string `json:"email"`
	}

	var payload loginPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}

	return strings.TrimSpace(payload.Email)
}

func extractAuthRefreshTokenFromRequestBody(body []byte) string {
	type refreshPayload struct {
		RefreshToken string `json:"refresh_token"`
	}

	var payload refreshPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}

	return strings.TrimSpace(payload.RefreshToken)
}

func extractSub2APITokensFromAuthResponse(body []byte) (string, *string, string, *int64, bool) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", nil, "", nil, false
	}

	candidates := []map[string]any{payload}
	if data, ok := payload["data"].(map[string]any); ok {
		candidates = append(candidates, data)
	}

	for _, candidate := range candidates {
		accessToken := pickString(candidate["access_token"], candidate["session_token"])
		if strings.TrimSpace(accessToken) == "" {
			continue
		}

		refresh := strings.TrimSpace(stringFromAny(candidate["refresh_token"]))
		var refreshPtr *string
		if refresh != "" {
			refreshPtr = &refresh
		}

		userEmail := strings.TrimSpace(stringFromAny(candidate["email"]))
		var upstreamUserID *int64

		if userObj, ok := candidate["user"].(map[string]any); ok {
			if userEmail == "" {
				userEmail = strings.TrimSpace(stringFromAny(userObj["email"]))
			}
			if id, ok := int64FromAny(userObj["id"]); ok {
				upstreamUserID = &id
			}
		}
		if upstreamUserID == nil {
			if id, ok := int64FromAny(candidate["id"]); ok {
				upstreamUserID = &id
			}
		}

		return accessToken, refreshPtr, userEmail, upstreamUserID, true
	}

	return "", nil, "", nil, false
}

type authIdentity struct {
	Email string
	Name  string
	Role  string
}

func extractAuthIdentityFromResponse(body []byte) authIdentity {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return authIdentity{}
	}

	candidates := []map[string]any{payload}
	if data, ok := payload["data"].(map[string]any); ok {
		candidates = append(candidates, data)
	}

	for _, candidate := range candidates {
		identity := authIdentity{
			Email: strings.TrimSpace(stringFromAny(candidate["email"])),
			Name:  strings.TrimSpace(stringFromAny(candidate["name"])),
			Role:  strings.TrimSpace(stringFromAny(candidate["role"])),
		}
		if userObj, ok := candidate["user"].(map[string]any); ok {
			if identity.Email == "" {
				identity.Email = strings.TrimSpace(stringFromAny(userObj["email"]))
			}
			if identity.Name == "" {
				identity.Name = strings.TrimSpace(stringFromAny(userObj["name"]))
			}
			if identity.Role == "" {
				identity.Role = strings.TrimSpace(stringFromAny(userObj["role"]))
			}
		}
		if identity.Email != "" {
			if identity.Role == "" {
				identity.Role = "user"
			}
			return identity
		}
	}

	return authIdentity{}
}

func pickString(values ...any) string {
	for _, value := range values {
		if v := strings.TrimSpace(stringFromAny(value)); v != "" {
			return v
		}
	}
	return ""
}

func stringFromAny(value any) string {
	v, ok := value.(string)
	if !ok {
		return ""
	}
	return v
}

func int64FromAny(value any) (int64, bool) {
	switch v := value.(type) {
	case float64:
		id := int64(v)
		if float64(id) == v {
			return id, true
		}
		return 0, false
	case int64:
		return v, true
	case int:
		return int64(v), true
	default:
		return 0, false
	}
}

func (r *routes) handleDashboardPassthrough(w http.ResponseWriter, req *http.Request, upstreamPath string) {
	if r.proxyClient == nil {
		writeError(w, http.StatusInternalServerError, "dashboard proxy is not configured")
		return
	}

	forwarded := req.Clone(req.Context())
	forwarded.Header = cloneHeaders(req.Header)
	forwarded.Header.Del("X-User-Id")
	if err := r.replaceAuthorizationWithStoredUpstreamToken(req.Context(), forwarded.Header); err != nil {
		if errors.Is(err, sub2apiauth.ErrTokenNotFound) {
			writeError(w, http.StatusUnauthorized, "upstream session unavailable")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve upstream session")
		return
	}

	resp, err := r.proxyClient.Do(req.Context(), forwarded, upstreamPath)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to proxy dashboard request")
		return
	}

	if err := proxy.CopyResponse(w, resp); err != nil {
		log.Printf("proxy dashboard response copy failed for %s: %v", upstreamPath, err)
		return
	}
}

func (r *routes) ensureLocalUser(ctx context.Context, identity authIdentity) (int64, bool, error) {
	email := strings.TrimSpace(identity.Email)
	if email == "" {
		return 0, false, nil
	}

	userID, found, err := r.findLocalUserIDByEmail(ctx, email)
	if err != nil {
		return 0, false, err
	}
	if found {
		return userID, true, nil
	}

	name := strings.TrimSpace(identity.Name)
	if name == "" {
		name = strings.TrimSpace(strings.Split(email, "@")[0])
		if name == "" {
			name = email
		}
	}
	role := strings.TrimSpace(identity.Role)
	if role == "" {
		role = "user"
	}
	if role != "user" && role != "admin" {
		role = "user"
	}

	userID, err = db.InsertID(ctx, r.sqlDialect, r.db, `INSERT INTO als_users(email, name, role) VALUES (?, ?, ?);`, "id", email, name, role)
	if err != nil {
		return 0, false, fmt.Errorf("create local auth user: %w", err)
	}
	return userID, true, nil
}

func (r *routes) createLocalSessionToken(ctx context.Context, userID int64) (string, error) {
	plaintext, tokenHash, err := auth.NewSessionToken()
	if err != nil {
		return "", fmt.Errorf("generate local session token: %w", err)
	}

	expiresAt := time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02 15:04:05")
	if _, err := r.db.ExecContext(ctx, db.Rebind(r.sqlDialect, `
		INSERT INTO als_sessions(user_id, token_hash, expires_at)
		VALUES (?, ?, ?);
	`), userID, tokenHash, expiresAt); err != nil {
		return "", fmt.Errorf("insert local session: %w", err)
	}

	return plaintext, nil
}

func injectSessionTokenIntoAuthResponse(body []byte, sessionToken string) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	payload["session_token"] = sessionToken
	if data, ok := payload["data"].(map[string]any); ok {
		data["session_token"] = sessionToken
		payload["data"] = data
	}
	return json.Marshal(payload)
}

func (r *routes) replaceAuthorizationWithStoredUpstreamToken(ctx context.Context, headers http.Header) error {
	if headers == nil || r.sub2apiAuth == nil {
		return nil
	}

	authHeader := strings.TrimSpace(headers.Get("Authorization"))
	if authHeader == "" {
		return nil
	}

	localSessionToken, err := extractBearerToken(authHeader)
	if err != nil {
		return nil
	}

	userID, found, err := r.findLocalUserIDBySessionToken(ctx, localSessionToken)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	upstreamAccessToken, err := r.sub2apiAuth.GetBearerTokenByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sub2apiauth.ErrTokenNotFound) {
			role, foundRole, roleErr := r.findLocalUserRoleByID(ctx, userID)
			if roleErr != nil {
				return roleErr
			}
			if foundRole && role == "admin" {
				return nil
			}
		}
		return err
	}

	headers.Set("Authorization", "Bearer "+upstreamAccessToken)
	return nil
}

func (r *routes) findLocalUserIDBySessionToken(ctx context.Context, sessionToken string) (int64, bool, error) {
	trimmed := strings.TrimSpace(sessionToken)
	if trimmed == "" {
		return 0, false, nil
	}

	tokenHash := auth.HashSessionToken(trimmed)
	var (
		userID    int64
		expiresAt time.Time
	)
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT user_id, expires_at
		FROM als_sessions
		WHERE token_hash = ?
		  AND revoked_at IS NULL
		LIMIT 1;
	`), tokenHash).Scan(&userID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("query local user by session token: %w", err)
	}
	if !expiresAt.After(time.Now().UTC()) {
		return 0, false, nil
	}

	return userID, true, nil
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
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if strings.TrimSpace(payload.Code) == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	wallet, err := r.userSvc.RedeemCard(req.Context(), authUser.ID, payload.Code)
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

	als_sessions, err := r.userSvc.ListSessions(req.Context(), authUser.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list als_sessions")
		return
	}

	response := listSessionsResponse{Sessions: make([]sessionResponse, 0, len(als_sessions))}
	for _, item := range als_sessions {
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
		FROM als_unit_prices up
		JOIN als_service_items si ON si.id = up.service_item_id
		LEFT JOIN als_tiers t ON t.id = up.tier_id
		WHERE up.service_item_id = ?
			AND up.effective_to IS NULL
	`
	args := []any{serviceItemID}
	if hasTierFilter {
		query += " AND up.tier_id = ?"
		args = append(args, tierID)
	}
	query += " ORDER BY up.id ASC;"

	rows, err := r.db.QueryContext(req.Context(), db.Rebind(r.sqlDialect, query), args...)
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
		if _, err := tx.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `
			UPDATE als_unit_prices
			SET effective_to = ?
			WHERE service_item_id = ?
				AND tier_id = ?
				AND effective_to IS NULL;
		`), now, serviceItemID, tierID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to set unit price")
			return
		}
	} else {
		if _, err := tx.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `
			UPDATE als_unit_prices
			SET effective_to = ?
			WHERE service_item_id = ?
				AND tier_id IS NULL
				AND effective_to IS NULL;
		`), now, serviceItemID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to set unit price")
			return
		}
	}

	var tierArg any
	if hasTier {
		tierArg = tierID
	}
	if _, err := tx.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `
		INSERT INTO als_unit_prices(service_item_id, tier_id, price_per_unit_micros, currency, effective_from)
		VALUES (?, ?, ?, ?, ?);
	`), serviceItemID, tierArg, payload.PricePerUnitMicros, currency, now); err != nil {
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
		result, err = r.db.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `
			UPDATE als_unit_prices
			SET effective_to = ?
			WHERE service_item_id = ?
				AND tier_id = ?
				AND effective_to IS NULL;
		`), now, serviceItemID, tierID)
	} else {
		result, err = r.db.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `
			UPDATE als_unit_prices
			SET effective_to = ?
			WHERE service_item_id = ?
				AND tier_id IS NULL
				AND effective_to IS NULL;
		`), now, serviceItemID)
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

// handlePublicTiers godoc
// @Summary List public als_tiers
// @Description List all public subscription als_tiers and included default service items.
// @Tags public
// @Produce json
// @Success 200 {object} listPublicTiersResponse
// @Failure 500 {object} errorResponse
// @Router /public/tiers [get]
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
		FROM als_tiers t
		LEFT JOIN als_tier_default_items tdi ON tdi.tier_id = t.id
		LEFT JOIN als_service_items si ON si.id = tdi.service_item_id
		ORDER BY t.id ASC, si.id ASC;
	`

	rows, err := r.db.QueryContext(req.Context(), query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list als_tiers")
		return
	}
	defer rows.Close()

	als_tiers := make([]publicTierResponse, 0)
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
			writeError(w, http.StatusInternalServerError, "failed to read als_tiers")
			return
		}

		idx, found := tierIndex[tierID]
		if !found {
			idx = len(als_tiers)
			tierIndex[tierID] = idx
			als_tiers = append(als_tiers, publicTierResponse{Code: tierCode, Name: tierName, DefaultItems: []publicTierItemResponse{}})
		}

		if itemCode.Valid {
			als_tiers[idx].DefaultItems = append(als_tiers[idx].DefaultItems, publicTierItemResponse{
				Code:          itemCode.String,
				Name:          itemName.String,
				Unit:          itemUnit.String,
				IncludedUnits: includedUnits.Int64,
			})
		}
	}

	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list als_tiers")
		return
	}

	writeJSON(w, http.StatusOK, listPublicTiersResponse{Tiers: als_tiers})
}

// handlePublicEstimate godoc
// @Summary Estimate tier price
// @Description Estimate total default price for a tier based on active unit prices.
// @Tags public
// @Accept json
// @Produce json
// @Param body body publicEstimateRequest true "Estimate payload"
// @Success 200 {object} publicEstimateResponse
// @Failure 400 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /public/estimate [post]
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
	err := r.db.QueryRowContext(req.Context(), db.Rebind(r.sqlDialect, `SELECT id, name FROM als_tiers WHERE code = ?;`), payload.TierCode).Scan(&tierID, &tierName)
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
		FROM als_tier_default_items tdi
		JOIN als_service_items si ON si.id = tdi.service_item_id
		WHERE tdi.tier_id = ?
		ORDER BY si.id ASC;
	`

	rows, err := r.db.QueryContext(req.Context(), db.Rebind(r.sqlDialect, itemsQuery), tierID)
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

// handlePublicListArticles godoc
// @Summary List published als_articles
// @Description List all published als_articles for public website.
// @Tags public
// @Produce json
// @Success 200 {object} publicArticleListResponse
func (r *routes) handlePublicListPackages(w http.ResponseWriter, req *http.Request) {
	rows, err := r.db.QueryContext(req.Context(), db.Rebind(r.sqlDialect, `
		SELECT code, name, price_micros, value_type, value_amount, description, features_json
		FROM als_tiers
		WHERE is_enabled = ?
		ORDER BY price_micros ASC;
	`), true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list packages")
		return
	}
	defer rows.Close()

	packages := make([]publicPackageResponse, 0)
	for rows.Next() {
		var (
			code         string
			name         string
			priceMicros  int64
			valueType    string
			valueAmount  int64
			description  string
			featuresJSON string
		)
		if err := rows.Scan(&code, &name, &priceMicros, &valueType, &valueAmount, &description, &featuresJSON); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list packages")
			return
		}
		packages = append(packages, publicPackageResponse{
			Code:        code,
			Name:        name,
			PriceMicros: priceMicros,
			ValueType:   valueType,
			ValueAmount: valueAmount,
			Description: description,
			Features:    parseFeaturesJSON(featuresJSON),
		})
	}

	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list packages")
		return
	}

	writeJSON(w, http.StatusOK, listPublicPackagesResponse{Packages: packages})
}

// @Failure 500 {object} errorResponse
// @Router /public/articles [get]
func (r *routes) handlePublicListArticles(w http.ResponseWriter, req *http.Request) {
	als_articles, err := r.articleSvc.ListPublishedArticles(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list als_articles")
		return
	}

	response := publicArticleListResponse{Articles: make([]publicArticleDTO, 0, len(als_articles))}
	for _, item := range als_articles {
		if item.PublishedAt == nil {
			continue
		}
		response.Articles = append(response.Articles, publicArticleDTO{
			Slug:            item.Slug,
			Title:           item.Title,
			Excerpt:         item.Excerpt,
			CoverImageURL:   item.CoverImageURL,
			Tag:             item.Tag,
			ReadTime:        item.ReadTime,
			AuthorName:      item.AuthorName,
			AuthorAvatarURL: item.AuthorAvatarURL,
			PublishedAt:     item.PublishedAt.UTC().Format(time.RFC3339),
		})
	}

	sort.SliceStable(response.Articles, func(i, j int) bool {
		return response.Articles[i].PublishedAt > response.Articles[j].PublishedAt
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handlePublicGetArticle godoc
// @Summary Get published article
// @Description Get one published article by slug.
// @Tags public
// @Produce json
// @Param slug path string true "Article slug"
// @Success 200 {object} publicArticleDetailResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /public/articles/{slug} [get]
func (r *routes) handlePublicGetArticle(w http.ResponseWriter, req *http.Request) {
	slug := strings.TrimSpace(req.PathValue("slug"))
	if slug == "" {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}

	item, err := r.articleSvc.GetArticleBySlug(req.Context(), slug)
	if errors.Is(err, article.ErrArticleNotFound) {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get article")
		return
	}
	if item.PublishedAt == nil {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(publicArticleDetailResponse{
		Article: publicArticleDetailDTO{
			Slug:            item.Slug,
			Title:           item.Title,
			Excerpt:         item.Excerpt,
			CoverImageURL:   item.CoverImageURL,
			Tag:             item.Tag,
			ReadTime:        item.ReadTime,
			AuthorName:      item.AuthorName,
			AuthorAvatarURL: item.AuthorAvatarURL,
			PublishedAt:     item.PublishedAt.UTC().Format(time.RFC3339),
			MDXBody:         item.MDXBody,
		},
	})
}

func (r *routes) handleAdminListArticles(w http.ResponseWriter, req *http.Request) {
	als_articles, err := r.articleSvc.ListArticles(req.Context(), article.ListArticlesFilters{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list als_articles")
		return
	}

	response := adminArticleListResponse{Articles: make([]adminArticleDTO, 0, len(als_articles))}
	for _, item := range als_articles {
		response.Articles = append(response.Articles, toAdminArticleDTO(item))
	}

	writeJSON(w, http.StatusOK, response)
}

func (r *routes) handleAdminCreateArticle(w http.ResponseWriter, req *http.Request) {
	adminUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var payload adminCreateArticleRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.Slug = strings.TrimSpace(payload.Slug)
	payload.Title = strings.TrimSpace(payload.Title)
	payload.MDXBody = strings.TrimSpace(payload.MDXBody)

	if payload.Slug == "" || payload.Title == "" || payload.MDXBody == "" {
		writeError(w, http.StatusBadRequest, "slug, title, and mdx_body are required")
		return
	}
	if err := validateArticleSlug(payload.Slug); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	status, err := normalizeAdminArticleStatus(payload.Status, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	adminUserID := adminUser.ID
	entry := &model.Article{
		Slug:            payload.Slug,
		Title:           payload.Title,
		Excerpt:         trimOptionalString(payload.Excerpt),
		CoverImageURL:   trimOptionalString(payload.CoverImageURL),
		Tag:             trimOptionalString(payload.Tag),
		ReadTime:        trimOptionalString(payload.ReadTime),
		AuthorName:      trimOptionalString(payload.AuthorName),
		AuthorAvatarURL: trimOptionalString(payload.AuthorAvatarURL),
		AuthorIcon:      trimOptionalString(payload.AuthorIcon),
		MDXBody:         payload.MDXBody,
		Status:          status,
		CreatedByUserID: &adminUserID,
		UpdatedByUserID: &adminUserID,
	}

	if err := r.articleSvc.CreateArticle(req.Context(), entry); err != nil {
		if errors.Is(err, article.ErrDuplicateSlug) {
			writeError(w, http.StatusConflict, "slug already exists")
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "article") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create article")
		return
	}

	writeJSON(w, http.StatusCreated, toAdminArticleDTO(*entry))
}

func (r *routes) handleAdminGetArticle(w http.ResponseWriter, req *http.Request) {
	slug := strings.TrimSpace(req.PathValue("slug"))
	if err := validateArticleSlug(slug); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	entry, err := r.findAdminArticleBySlug(req.Context(), slug)
	if errors.Is(err, article.ErrArticleNotFound) {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get article")
		return
	}

	writeJSON(w, http.StatusOK, toAdminArticleDTO(*entry))
}

func (r *routes) handleAdminUpdateArticle(w http.ResponseWriter, req *http.Request) {
	adminUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	slug := strings.TrimSpace(req.PathValue("slug"))
	if err := validateArticleSlug(slug); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := r.findAdminArticleBySlug(req.Context(), slug)
	if errors.Is(err, article.ErrArticleNotFound) {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update article")
		return
	}

	var payload adminUpdateArticleRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if payload.Slug != nil {
		trimmed := strings.TrimSpace(*payload.Slug)
		if err := validateArticleSlug(trimmed); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		payload.Slug = &trimmed
	}

	if payload.Title != nil {
		trimmed := strings.TrimSpace(*payload.Title)
		if trimmed == "" {
			writeError(w, http.StatusBadRequest, "title cannot be empty")
			return
		}
		payload.Title = &trimmed
	}

	if payload.MDXBody != nil {
		trimmed := strings.TrimSpace(*payload.MDXBody)
		if trimmed == "" {
			writeError(w, http.StatusBadRequest, "mdx_body cannot be empty")
			return
		}
		payload.MDXBody = &trimmed
	}

	status := ""
	if payload.Status != nil {
		status, err = normalizeAdminArticleStatus(*payload.Status, false)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if !isValidAdminStatusTransition(existing.Status, status) {
			writeError(w, http.StatusBadRequest, "invalid status transition")
			return
		}
	}

	adminUserID := adminUser.ID
	updatedArticle := &model.Article{
		LegacyID:        payload.LegacyID,
		Slug:            optionalStringValue(payload.Slug),
		Title:           optionalStringValue(payload.Title),
		Excerpt:         trimOptionalString(payload.Excerpt),
		CoverImageURL:   trimOptionalString(payload.CoverImageURL),
		Tag:             trimOptionalString(payload.Tag),
		ReadTime:        trimOptionalString(payload.ReadTime),
		AuthorName:      trimOptionalString(payload.AuthorName),
		AuthorAvatarURL: trimOptionalString(payload.AuthorAvatarURL),
		AuthorIcon:      trimOptionalString(payload.AuthorIcon),
		MDXBody:         optionalStringValue(payload.MDXBody),
		Status:          status,
		UpdatedByUserID: &adminUserID,
	}

	if err := r.articleSvc.UpdateArticle(req.Context(), slug, updatedArticle); err != nil {
		if errors.Is(err, article.ErrArticleNotFound) {
			writeError(w, http.StatusNotFound, "article not found")
			return
		}
		if errors.Is(err, article.ErrDuplicateSlug) {
			writeError(w, http.StatusConflict, "slug already exists")
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "invalid article status") {
			writeError(w, http.StatusBadRequest, "invalid article status")
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "article") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update article")
		return
	}

	refreshed, err := r.articleSvc.GetArticleByID(req.Context(), updatedArticle.ID)
	if errors.Is(err, article.ErrArticleNotFound) {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update article")
		return
	}

	writeJSON(w, http.StatusOK, toAdminArticleDTO(*refreshed))
}

func (r *routes) handleAdminDeleteArticle(w http.ResponseWriter, req *http.Request) {
	slug := strings.TrimSpace(req.PathValue("slug"))
	if err := validateArticleSlug(slug); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := r.articleSvc.DeleteArticle(req.Context(), slug); err != nil {
		if errors.Is(err, article.ErrArticleNotFound) {
			writeError(w, http.StatusNotFound, "article not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete article")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (r *routes) handleAdminPublishArticle(w http.ResponseWriter, req *http.Request) {
	slug := strings.TrimSpace(req.PathValue("slug"))
	if slug == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}

	if err := r.articleSvc.PublishArticle(req.Context(), slug); err != nil {
		if errors.Is(err, article.ErrArticleNotFound) {
			writeError(w, http.StatusNotFound, "article not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to publish article")
		return
	}

	updated, err := r.findAdminArticleBySlug(req.Context(), slug)
	if errors.Is(err, article.ErrArticleNotFound) {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve updated article")
		return
	}

	writeJSON(w, http.StatusOK, toAdminArticleDTO(*updated))
}

func (r *routes) handleAdminUnpublishArticle(w http.ResponseWriter, req *http.Request) {
	slug := strings.TrimSpace(req.PathValue("slug"))
	if slug == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}

	if err := r.articleSvc.UnpublishArticle(req.Context(), slug); err != nil {
		if errors.Is(err, article.ErrArticleNotFound) {
			writeError(w, http.StatusNotFound, "article not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to unpublish article")
		return
	}

	updated, err := r.findAdminArticleBySlug(req.Context(), slug)
	if errors.Is(err, article.ErrArticleNotFound) {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve updated article")
		return
	}

	writeJSON(w, http.StatusOK, toAdminArticleDTO(*updated))
}

func (r *routes) handleCreatePackageCheckoutSession(w http.ResponseWriter, req *http.Request) {
	if r.stripeClient == nil {
		writeError(w, http.StatusServiceUnavailable, "stripe checkout is not configured")
		return
	}

	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var payload createPackageCheckoutSessionRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	payload.TierCode = strings.TrimSpace(payload.TierCode)
	if payload.TierCode == "" {
		writeError(w, http.StatusBadRequest, "tier_code is required")
		return
	}

	pkg, err := r.loadAdminPackageByCode(req.Context(), payload.TierCode)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "package not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load package")
		return
	}
	if !pkg.IsEnabled {
		writeError(w, http.StatusBadRequest, "package is not enabled")
		return
	}
	amountMinor, err := microsToCurrencyMinor(pkg.PriceMicros)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	customerEmail := strings.TrimSpace(authUser.Email)
	if customerEmail == "" {
		profile, err := r.userSvc.GetProfile(req.Context(), authUser.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load profile")
			return
		}
		customerEmail = strings.TrimSpace(profile.Email)
	}

	session, err := r.stripeClient.CreateCheckoutSession(req.Context(), portalstripe.CheckoutSessionInput{
		PackageCode:   pkg.Code,
		PackageName:   pkg.Name,
		UserID:        authUser.ID,
		CustomerEmail: customerEmail,
		AmountMinor:   amountMinor,
	})
	if err != nil {
		log.Printf("stripe checkout session creation failed for user_id=%d tier_code=%s: %v", authUser.ID, pkg.Code, err)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to create stripe checkout session: %v", err))
		return
	}
	if err := r.recordCheckoutSession(req.Context(), "stripe", session.ID, authUser.ID, pkg, customerEmail, amountMinor, r.stripeClient.Currency()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist checkout session")
		return
	}

	writeJSON(w, http.StatusOK, createPackageCheckoutSessionResponse{
		SessionID:   session.ID,
		CheckoutURL: session.URL,
	})
}

func (r *routes) handleStripeWebhook(w http.ResponseWriter, req *http.Request) {
	if r.stripeClient == nil {
		writeError(w, http.StatusServiceUnavailable, "stripe checkout is not configured")
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read stripe webhook body")
		return
	}

	event, err := r.stripeClient.ConstructEvent(body, req.Header.Get("Stripe-Signature"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid stripe webhook signature")
		return
	}

	switch event.Type {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded":
	default:
		writeJSON(w, http.StatusOK, map[string]bool{"received": true})
		return
	}

	var session portalstripe.CheckoutSessionCompleted
	if err := json.Unmarshal(event.Data.Object, &session); err != nil {
		writeError(w, http.StatusBadRequest, "invalid stripe checkout session payload")
		return
	}
	if strings.TrimSpace(session.PaymentStatus) != "paid" {
		writeJSON(w, http.StatusOK, map[string]bool{"received": true})
		return
	}

	userID, err := parsePositiveInt64(session.Metadata["user_id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "stripe checkout session metadata user_id is invalid")
		return
	}
	tierCode := strings.TrimSpace(session.Metadata["tier_code"])
	if tierCode == "" {
		writeError(w, http.StatusBadRequest, "stripe checkout session metadata tier_code is required")
		return
	}

	payloadBytes, _ := json.Marshal(map[string]any{
		"stripe_event_type":  event.Type,
		"stripe_checkout_id": session.ID,
		"payment_status":     session.PaymentStatus,
		"currency":           session.Currency,
		"amount_total":       session.AmountTotal,
		"customer_email":     session.CustomerEmail,
	})

	job, err := r.ingestAndMaybeExecutePaymentSuccess(req.Context(), adminPaymentSuccessRequest{
		PaymentEventID: event.ID,
		OrderID:        strings.TrimSpace(session.ID),
		Provider:       "stripe",
		UserID:         userID,
		TierCode:       tierCode,
		Payload:        payloadBytes,
	}, "stripe:"+event.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to process stripe checkout completion")
		return
	}
	if err := r.markCheckoutSessionCompleted(req.Context(), "stripe", session.ID, event.ID, tierCode, userID, session.CustomerEmail, session.AmountTotal, session.Currency, job.ID, body); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist stripe payment record")
		return
	}

	writeJSON(w, http.StatusOK, adminPaymentSuccessResponse{Job: toFulfillmentJobResponse(job)})
}

func (r *routes) handleGetPackageCheckoutStatus(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	sessionID := strings.TrimSpace(req.URL.Query().Get("session_id"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	record, fulfillmentJob, err := r.loadCheckoutStatus(req.Context(), authUser.ID, sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "checkout session not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load checkout session status")
		return
	}
	if fulfillmentJob != nil && fulfillmentJob.Status == fulfillment.StatusFailedRetryable && !fulfillmentJob.AvailableAt.After(time.Now().UTC()) {
		replayedJob, replayErr := r.retryPaymentFulfillmentJob(req.Context(), fulfillmentJob, "checkout_status_auto_retry")
		if replayErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to retry checkout fulfillment")
			return
		}
		fulfillmentJob = replayedJob
	}

	response := checkoutPackageStatusResponse{
		Status:            deriveCheckoutStatus(record.Status, fulfillmentJob),
		Provider:          record.Provider,
		CheckoutSessionID: record.CheckoutSessionID,
		TierCode:          record.TierCode,
		PackageName:       record.PackageName,
		AmountMinor:       record.AmountMinor,
		Currency:          record.Currency,
	}
	if record.PaymentEventID != nil && strings.TrimSpace(*record.PaymentEventID) != "" {
		response.PaymentEventID = record.PaymentEventID
	}
	if fulfillmentJob != nil {
		payload := toFulfillmentJobResponse(fulfillmentJob)
		response.FulfillmentJob = &payload
	}

	writeJSON(w, http.StatusOK, response)
}

func (r *routes) handleAdminPaymentSuccess(w http.ResponseWriter, req *http.Request) {
	if r.fulfillmentSvc == nil {
		writeError(w, http.StatusInternalServerError, "fulfillment service is not configured")
		return
	}

	idempotencyKey := strings.TrimSpace(req.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "idempotency key is required")
		return
	}

	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()

	var payload adminPaymentSuccessRequest
	if err := decoder.Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.PaymentEventID = strings.TrimSpace(payload.PaymentEventID)
	payload.OrderID = strings.TrimSpace(payload.OrderID)
	payload.Provider = strings.TrimSpace(payload.Provider)
	if payload.PaymentEventID == "" {
		writeError(w, http.StatusBadRequest, "payment_event_id is required")
		return
	}
	if payload.UserID <= 0 {
		writeError(w, http.StatusBadRequest, "user_id must be positive")
		return
	}
	if payload.SubscriptionID != nil && *payload.SubscriptionID <= 0 {
		writeError(w, http.StatusBadRequest, "subscription_id must be positive")
		return
	}

	job, err := r.ingestAndMaybeExecutePaymentSuccess(req.Context(), payload, idempotencyKey)
	if err != nil {
		if errors.Is(err, fulfillment.ErrIdempotencyConflict) {
			writeError(w, http.StatusConflict, "idempotency key already used with different payment payload")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to execute payment fulfillment")
		return
	}

	writeJSON(w, http.StatusAccepted, adminPaymentSuccessResponse{Job: toFulfillmentJobResponse(job)})
}

func (r *routes) ingestAndMaybeExecutePaymentSuccess(ctx context.Context, payload adminPaymentSuccessRequest, idempotencyKey string) (*fulfillment.Job, error) {
	normalizedPayload, err := json.Marshal(struct {
		PaymentEventID  string                          `json:"payment_event_id"`
		OrderID         string                          `json:"order_id,omitempty"`
		Provider        string                          `json:"provider,omitempty"`
		BalanceRecharge *proxy.UpdateUserBalanceRequest `json:"balance_recharge,omitempty"`
		APIKey          *proxy.CreateUserAPIKeyRequest  `json:"api_key,omitempty"`
		TierCode        string                          `json:"tier_code,omitempty"`
		Payload         json.RawMessage                 `json:"payload,omitempty"`
	}{
		PaymentEventID:  payload.PaymentEventID,
		OrderID:         payload.OrderID,
		Provider:        payload.Provider,
		BalanceRecharge: payload.BalanceRecharge,
		APIKey:          payload.APIKey,
		TierCode:        strings.TrimSpace(payload.TierCode),
		Payload:         payload.Payload,
	})
	if err != nil {
		return nil, fmt.Errorf("normalize payment event: %w", err)
	}

	job, err := r.fulfillmentSvc.CreateOrLoadJobByIdempotency(ctx, &fulfillment.CreateJobInput{
		UserID:         &payload.UserID,
		SubscriptionID: payload.SubscriptionID,
		EventType:      "payment_succeeded",
		PayloadJSON:    string(normalizedPayload),
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return nil, err
	}

	if shouldExecutePaymentSuccessFulfillment(payload) {
		return r.executePaymentSuccessFulfillment(ctx, job, payload, idempotencyKey)
	}
	return job, nil
}

func shouldExecutePaymentSuccessFulfillment(payload adminPaymentSuccessRequest) bool {
	return payload.BalanceRecharge != nil || payload.APIKey != nil || payload.TierCode != ""
}

type paymentRecord struct {
	ID                int64
	Provider          string
	CheckoutSessionID string
	PaymentEventID    *string
	UserID            int64
	TierCode          string
	PackageName       string
	AmountMinor       int64
	Currency          string
	Status            string
	FulfillmentJobID  *int64
}

func (r *routes) recordCheckoutSession(ctx context.Context, provider, checkoutSessionID string, userID int64, pkg adminPackageResponse, customerEmail string, amountMinor int64, currency string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	payloadJSON, _ := json.Marshal(map[string]any{
		"checkout_session_id": checkoutSessionID,
		"tier_code":           pkg.Code,
	})
	_, err := r.db.ExecContext(ctx, db.Rebind(r.sqlDialect, `
		INSERT INTO als_payment_records(
			provider,
			checkout_session_id,
			user_id,
			tier_code,
			package_name,
			customer_email,
			amount_minor,
			currency,
			status,
			payload_json,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`), provider, checkoutSessionID, userID, pkg.Code, pkg.Name, strings.TrimSpace(customerEmail), amountMinor, strings.ToLower(strings.TrimSpace(currency)), "checkout_created", string(payloadJSON), now, now)
	return err
}

func (r *routes) markCheckoutSessionCompleted(ctx context.Context, provider, checkoutSessionID, paymentEventID, tierCode string, userID int64, customerEmail string, amountMinor int64, currency string, fulfillmentJobID int64, payload []byte) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := r.db.ExecContext(ctx, db.Rebind(r.sqlDialect, `
		UPDATE als_payment_records
		SET payment_event_id = ?,
			user_id = ?,
			tier_code = ?,
			customer_email = ?,
			amount_minor = ?,
			currency = ?,
			status = ?,
			fulfillment_job_id = ?,
			payload_json = ?,
			completed_at = ?,
			updated_at = ?
		WHERE provider = ? AND checkout_session_id = ?;
	`), paymentEventID, userID, tierCode, strings.TrimSpace(customerEmail), amountMinor, strings.ToLower(strings.TrimSpace(currency)), "payment_succeeded", fulfillmentJobID, string(payload), now, now, provider, checkoutSessionID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected > 0 {
		return nil
	}
	_, err = r.db.ExecContext(ctx, db.Rebind(r.sqlDialect, `
		INSERT INTO als_payment_records(
			provider,
			checkout_session_id,
			payment_event_id,
			user_id,
			tier_code,
			customer_email,
			amount_minor,
			currency,
			status,
			fulfillment_job_id,
			payload_json,
			completed_at,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`), provider, checkoutSessionID, paymentEventID, userID, tierCode, strings.TrimSpace(customerEmail), amountMinor, strings.ToLower(strings.TrimSpace(currency)), "payment_succeeded", fulfillmentJobID, string(payload), now, now, now)
	return err
}

func (r *routes) loadCheckoutStatus(ctx context.Context, userID int64, checkoutSessionID string) (*paymentRecord, *fulfillment.Job, error) {
	var (
		record           paymentRecord
		paymentEventID   sql.NullString
		fulfillmentJobID sql.NullInt64
	)
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT
			id,
			provider,
			checkout_session_id,
			payment_event_id,
			user_id,
			tier_code,
			package_name,
			amount_minor,
			currency,
			status,
			fulfillment_job_id
		FROM als_payment_records
		WHERE user_id = ? AND checkout_session_id = ?
		LIMIT 1;
	`), userID, checkoutSessionID).Scan(
		&record.ID,
		&record.Provider,
		&record.CheckoutSessionID,
		&paymentEventID,
		&record.UserID,
		&record.TierCode,
		&record.PackageName,
		&record.AmountMinor,
		&record.Currency,
		&record.Status,
		&fulfillmentJobID,
	)
	if err != nil {
		return nil, nil, err
	}
	if paymentEventID.Valid {
		value := paymentEventID.String
		record.PaymentEventID = &value
	}
	if fulfillmentJobID.Valid {
		value := fulfillmentJobID.Int64
		record.FulfillmentJobID = &value
	}

	var job *fulfillment.Job
	if record.FulfillmentJobID != nil && r.fulfillmentSvc != nil {
		job, err = r.fulfillmentSvc.GetJobByID(ctx, *record.FulfillmentJobID)
		if err != nil && !errors.Is(err, fulfillment.ErrJobNotFound) {
			return nil, nil, err
		}
	}

	return &record, job, nil
}

func deriveCheckoutStatus(paymentStatus string, job *fulfillment.Job) string {
	paymentStatus = strings.TrimSpace(paymentStatus)
	switch {
	case job == nil:
		if paymentStatus == "payment_succeeded" {
			return "processing"
		}
		return "pending"
	case job.Status == fulfillment.StatusFulfilled:
		return "fulfilled"
	case job.Status == fulfillment.StatusFailedTerminal:
		return "failed"
	case job.Status == fulfillment.StatusFailedRetryable:
		return "retrying"
	default:
		return "processing"
	}
}

func (r *routes) retryPaymentFulfillmentJob(ctx context.Context, job *fulfillment.Job, eventType string) (*fulfillment.Job, error) {
	if job == nil {
		return nil, errors.New("fulfillment job is required")
	}
	if job.Status != fulfillment.StatusFailedRetryable {
		return job, nil
	}
	if job.IdempotencyKey == nil || strings.TrimSpace(*job.IdempotencyKey) == "" {
		return nil, errors.New("fulfillment job is missing idempotency key")
	}

	var payload adminPaymentSuccessRequest
	if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
		return nil, fmt.Errorf("decode fulfillment payload: %w", err)
	}
	if payload.UserID <= 0 && job.UserID != nil {
		payload.UserID = *job.UserID
	}
	if payload.SubscriptionID == nil && job.SubscriptionID != nil {
		subscriptionID := *job.SubscriptionID
		payload.SubscriptionID = &subscriptionID
	}
	if payload.UserID <= 0 {
		return nil, errors.New("fulfillment payload is missing user_id")
	}
	if !shouldExecutePaymentSuccessFulfillment(payload) {
		return nil, errors.New("fulfillment job has no replayable side effects")
	}

	retryCount := job.RetryCount
	resetPayload := fmt.Sprintf(`{"outcome":%q}`, eventType)
	replayedJob, err := r.fulfillmentSvc.TransitionJob(ctx, job.ID, &fulfillment.TransitionInput{
		Status:       fulfillment.StatusPaidUnfulfilled,
		ErrorMessage: nil,
		EventType:    eventType,
		RetryCount:   &retryCount,
		EventPayload: &resetPayload,
	})
	if err != nil {
		return nil, err
	}

	return r.executePaymentSuccessFulfillment(ctx, replayedJob, payload, strings.TrimSpace(*replayedJob.IdempotencyKey))
}

func (r *routes) listAdminPaymentRecords(ctx context.Context, limit int) ([]adminPaymentRecordResponse, error) {
	rows, err := r.db.QueryContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT
			id,
			provider,
			checkout_session_id,
			payment_event_id,
			user_id,
			tier_code,
			package_name,
			amount_minor,
			currency,
			status,
			fulfillment_job_id
		FROM als_payment_records
		ORDER BY id DESC
		LIMIT ?;
	`), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]adminPaymentRecordResponse, 0)
	for rows.Next() {
		var (
			record         adminPaymentRecordResponse
			paymentEventID sql.NullString
			fulfillmentID  sql.NullInt64
		)
		if err := rows.Scan(
			&record.ID,
			&record.Provider,
			&record.CheckoutSessionID,
			&paymentEventID,
			&record.UserID,
			&record.TierCode,
			&record.PackageName,
			&record.AmountMinor,
			&record.Currency,
			&record.Status,
			&fulfillmentID,
		); err != nil {
			return nil, err
		}
		if paymentEventID.Valid && strings.TrimSpace(paymentEventID.String) != "" {
			value := paymentEventID.String
			record.PaymentEventID = &value
		}
		if fulfillmentID.Valid && r.fulfillmentSvc != nil {
			job, err := r.fulfillmentSvc.GetJobByID(ctx, fulfillmentID.Int64)
			if err != nil && !errors.Is(err, fulfillment.ErrJobNotFound) {
				return nil, err
			}
			if job != nil {
				payload := toFulfillmentJobResponse(job)
				record.FulfillmentJob = &payload
				record.OrderStatus = deriveCheckoutStatus(record.Status, job)
				record.Replayable = job.Status == fulfillment.StatusFailedRetryable
			}
		}
		if record.OrderStatus == "" {
			record.OrderStatus = deriveCheckoutStatus(record.Status, nil)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *routes) executePaymentSuccessFulfillment(ctx context.Context, job *fulfillment.Job, payload adminPaymentSuccessRequest, parentIdempotencyKey string) (*fulfillment.Job, error) {
	if job == nil {
		return nil, errors.New("fulfillment job is required")
	}
	if r.proxyClient == nil {
		return nil, errors.New("proxy client is not configured")
	}
	if payload.APIKey != nil && r.sub2apiAuth == nil {
		return nil, errors.New("sub2api auth service is not configured")
	}

	switch job.Status {
	case fulfillment.StatusFulfilled, fulfillment.StatusFailedTerminal:
		return job, nil
	}

	if payload.BalanceRecharge != nil {
		balanceErr := r.executeBalanceRecharge(ctx, job, payload, parentIdempotencyKey)
		refreshedJob, refreshErr := r.fulfillmentSvc.GetJobByID(ctx, job.ID)
		if refreshErr != nil {
			return nil, refreshErr
		}
		job = refreshedJob
		if balanceErr != nil {
			if job != nil {
				return job, nil
			}
			return nil, balanceErr
		}
	}

	if payload.APIKey != nil {
		apiKeyErr := r.executeDelegatedAPIKeyCreation(ctx, job, payload, parentIdempotencyKey)
		refreshedJob, refreshErr := r.fulfillmentSvc.GetJobByID(ctx, job.ID)
		if refreshErr != nil {
			return nil, refreshErr
		}
		job = refreshedJob
		if apiKeyErr != nil {
			if job != nil {
				return job, nil
			}
			return nil, apiKeyErr
		}
	}

	if strings.TrimSpace(payload.TierCode) != "" {
		packageErr := r.executePackagePurchaseFulfillment(ctx, job, payload, parentIdempotencyKey)
		refreshedJob, refreshErr := r.fulfillmentSvc.GetJobByID(ctx, job.ID)
		if refreshErr != nil {
			return nil, refreshErr
		}
		job = refreshedJob
		if packageErr != nil {
			if job != nil {
				return job, nil
			}
			return nil, packageErr
		}
	}

	if job == nil {
		return nil, errors.New("fulfillment job became nil after execution")
	}
	refreshed, err := r.fulfillmentSvc.GetJobByID(ctx, job.ID)
	if err != nil {
		return nil, err
	}
	return refreshed, nil
}

func (r *routes) executeBalanceRecharge(ctx context.Context, job *fulfillment.Job, payload adminPaymentSuccessRequest, parentIdempotencyKey string) error {
	if payload.BalanceRecharge == nil {
		return nil
	}

	childKey := parentIdempotencyKey + ":balance"
	_, err := r.proxyClient.UpdateUserBalance(ctx, payload.UserID, *payload.BalanceRecharge, childKey)
	if err != nil {
		_, applyErr := r.fulfillmentSvc.ApplyBalanceRechargeResult(ctx, job.ID, err)
		if applyErr != nil {
			return applyErr
		}
		return nil
	}

	if payload.APIKey == nil {
		_, applyErr := r.fulfillmentSvc.ApplyBalanceRechargeResult(ctx, job.ID, nil)
		return applyErr
	}

	payloadJSON := `{"outcome":"fulfilled"}`
	_, transitionErr := r.fulfillmentSvc.TransitionJob(ctx, job.ID, &fulfillment.TransitionInput{
		Status:       fulfillment.StatusPaidUnfulfilled,
		EventType:    "sub2api_balance_recharge_succeeded",
		RetryCount:   &job.RetryCount,
		EventPayload: &payloadJSON,
	})
	return transitionErr
}

func (r *routes) executeDelegatedAPIKeyCreation(ctx context.Context, job *fulfillment.Job, payload adminPaymentSuccessRequest, parentIdempotencyKey string) error {
	if payload.APIKey == nil {
		return nil
	}

	bearerToken, err := r.sub2apiAuth.GetBearerTokenByUserID(ctx, payload.UserID)
	if err != nil {
		_, applyErr := r.fulfillmentSvc.ApplyAPIKeyCreationResult(ctx, job.ID, err)
		if applyErr != nil {
			return applyErr
		}
		return nil
	}

	childKey := parentIdempotencyKey + ":api_key"
	_, err = r.proxyClient.CreateUserAPIKey(ctx, bearerToken, *payload.APIKey, childKey)
	_, applyErr := r.fulfillmentSvc.ApplyAPIKeyCreationResult(ctx, job.ID, err)
	if applyErr != nil {
		return applyErr
	}
	return nil
}

func (r *routes) executePackagePurchaseFulfillment(ctx context.Context, job *fulfillment.Job, payload adminPaymentSuccessRequest, parentIdempotencyKey string) error {
	pkg, err := r.loadAdminPackageByCode(ctx, strings.TrimSpace(payload.TierCode))
	if err != nil {
		_, applyErr := r.fulfillmentSvc.ApplyCreateAndRedeemResult(ctx, job.ID, err)
		if applyErr != nil {
			return applyErr
		}
		return nil
	}
	if !pkg.IsEnabled {
		_, applyErr := r.fulfillmentSvc.ApplyCreateAndRedeemResult(ctx, job.ID, errors.New("package is not enabled"))
		if applyErr != nil {
			return applyErr
		}
		return nil
	}
	if len(pkg.GroupIDs) == 0 {
		_, applyErr := r.fulfillmentSvc.ApplyCreateAndRedeemResult(ctx, job.ID, errors.New("package has no bound groups"))
		if applyErr != nil {
			return applyErr
		}
		return nil
	}

	if _, _, err := r.ensureActiveSubscriptionForUser(ctx, payload.UserID, pkg.Code); err != nil {
		_, applyErr := r.fulfillmentSvc.ApplyCreateAndRedeemResult(ctx, job.ID, err)
		if applyErr != nil {
			return applyErr
		}
		return nil
	}

	var redeemErr error
	switch strings.TrimSpace(pkg.ValueType) {
	case "days":
		if pkg.ValueAmount <= 0 {
			redeemErr = errors.New("package days value must be positive")
			break
		}
		validityDays := int(pkg.ValueAmount)
		for _, groupID := range pkg.GroupIDs {
			childKey := parentIdempotencyKey + ":package:" + strconv.FormatInt(groupID, 10)
			groupIDCopy := groupID
			_, redeemErr = r.proxyClient.CreateAndRedeem(ctx, proxy.CreateAndRedeemRequest{
				Code:         buildRedeemCode(payload.OrderID, payload.PaymentEventID, groupID),
				Type:         proxy.RedeemTypeSubscription,
				Value:        float64(validityDays),
				UserID:       payload.UserID,
				GroupID:      &groupIDCopy,
				ValidityDays: &validityDays,
				Notes:        fmt.Sprintf("stripe package purchase %s", pkg.Code),
			}, childKey)
			if redeemErr != nil {
				break
			}
		}
	case "balance":
		amount := microsToFloatCurrency(pkg.ValueAmount)
		_, redeemErr = r.proxyClient.CreateAndRedeem(ctx, proxy.CreateAndRedeemRequest{
			Code:   buildRedeemCode(payload.OrderID, payload.PaymentEventID, 0),
			Type:   proxy.RedeemTypeBalance,
			Value:  amount,
			UserID: payload.UserID,
			Notes:  fmt.Sprintf("stripe package purchase %s", pkg.Code),
		}, parentIdempotencyKey+":package:balance")
	default:
		redeemErr = errors.New("package value_type is not supported for fulfillment")
	}

	_, applyErr := r.fulfillmentSvc.ApplyCreateAndRedeemResult(ctx, job.ID, redeemErr)
	if applyErr != nil {
		return applyErr
	}
	return nil
}

func (r *routes) handleAdminGetFulfillmentJob(w http.ResponseWriter, req *http.Request) {
	if r.fulfillmentSvc == nil {
		writeError(w, http.StatusInternalServerError, "fulfillment service is not configured")
		return
	}

	jobID, err := parsePathID(req.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid fulfillment job id")
		return
	}

	job, err := r.fulfillmentSvc.GetJobByID(req.Context(), jobID)
	if errors.Is(err, fulfillment.ErrJobNotFound) {
		writeError(w, http.StatusNotFound, "fulfillment job not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get fulfillment job")
		return
	}

	writeJSON(w, http.StatusOK, adminPaymentSuccessResponse{Job: toFulfillmentJobResponse(job)})
}

func (r *routes) handleAdminReplayFulfillmentJob(w http.ResponseWriter, req *http.Request) {
	if r.fulfillmentSvc == nil {
		writeError(w, http.StatusInternalServerError, "fulfillment service is not configured")
		return
	}

	jobID, err := parsePathID(req.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid fulfillment job id")
		return
	}

	job, err := r.fulfillmentSvc.GetJobByID(req.Context(), jobID)
	if errors.Is(err, fulfillment.ErrJobNotFound) {
		writeError(w, http.StatusNotFound, "fulfillment job not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get fulfillment job")
		return
	}
	if job.Status != fulfillment.StatusFailedRetryable {
		writeError(w, http.StatusBadRequest, "only retryable fulfillment jobs can be replayed")
		return
	}
	job, err = r.retryPaymentFulfillmentJob(req.Context(), job, "admin_replay_requested")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to replay fulfillment job")
		return
	}

	writeJSON(w, http.StatusAccepted, adminPaymentSuccessResponse{Job: toFulfillmentJobResponse(job)})
}

func (r *routes) handleGetFulfillmentJob(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if r.fulfillmentSvc == nil {
		writeError(w, http.StatusInternalServerError, "fulfillment service is not configured")
		return
	}

	jobID, err := parsePathID(req.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid fulfillment job id")
		return
	}

	job, err := r.fulfillmentSvc.GetJobByID(req.Context(), jobID)
	if errors.Is(err, fulfillment.ErrJobNotFound) {
		writeError(w, http.StatusNotFound, "fulfillment job not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get fulfillment job")
		return
	}
	if job.UserID == nil || *job.UserID != authUser.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	writeJSON(w, http.StatusOK, adminPaymentSuccessResponse{Job: toFulfillmentJobResponse(job)})
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
	_, _, tierName, quotas, err := r.createOrReplaceSubscription(req.Context(), user.ID, payload)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "tier not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create subscription")
		return
	}

	writeJSON(w, http.StatusCreated, getSubscriptionResponse{Subscription: subscriptionResponse{
		TierCode: payload.TierCode,
		TierName: tierName,
		Quotas:   quotas,
	}})
}

func (r *routes) createOrReplaceSubscription(ctx context.Context, userID int64, payload createSubscriptionRequest) (int64, int64, string, []subscriptionQuotaResponse, error) {
	tierID, tierName, err := r.lookupTier(ctx, payload.TierCode)
	if err != nil {
		return 0, 0, "", nil, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, "", nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, db.Rebind(r.sqlDialect, `
		UPDATE als_subscriptions
		SET status = 'ended', ended_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND status = 'active' AND ended_at IS NULL;
	`), userID); err != nil {
		return 0, 0, "", nil, err
	}

	subscriptionID, err := db.InsertID(ctx, r.sqlDialect, tx, `
		INSERT INTO als_subscriptions(user_id, tier_id, status, started_at)
		VALUES (?, ?, 'active', CURRENT_TIMESTAMP)
	`, "id", userID, tierID)
	if err != nil {
		return 0, 0, "", nil, err
	}

	seenCodes := make(map[string]struct{})
	for _, override := range payload.Overrides {
		code := strings.TrimSpace(override.ServiceItemCode)
		if code == "" {
			return 0, 0, "", nil, errors.New("override service_item_code is required")
		}
		if override.IncludedUnits < 0 {
			return 0, 0, "", nil, errors.New("override included_units must be non-negative")
		}
		if _, exists := seenCodes[code]; exists {
			return 0, 0, "", nil, errors.New("duplicate override service_item_code")
		}
		seenCodes[code] = struct{}{}

		serviceItemID, err := lookupServiceItemID(ctx, tx, r.sqlDialect, code)
		if err != nil {
			return 0, 0, "", nil, err
		}

		if _, err := tx.ExecContext(ctx, db.Rebind(r.sqlDialect, `
			INSERT INTO als_subscription_overrides(subscription_id, service_item_id, included_units)
			VALUES (?, ?, ?);
		`), subscriptionID, serviceItemID, override.IncludedUnits); err != nil {
			return 0, 0, "", nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, "", nil, err
	}

	quotas, err := r.loadEffectiveQuotas(ctx, tierID, subscriptionID)
	if err != nil {
		return 0, 0, "", nil, err
	}

	return subscriptionID, tierID, tierName, quotas, nil
}

func (r *routes) ensureActiveSubscriptionForUser(ctx context.Context, userID int64, tierCode string) (int64, string, error) {
	current, found, err := r.loadActiveSubscription(ctx, userID)
	if err == nil && found && current.TierCode == tierCode {
		var subscriptionID int64
		err = r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
			SELECT id
			FROM als_subscriptions
			WHERE user_id = ? AND status = 'active' AND ended_at IS NULL
			ORDER BY id DESC
			LIMIT 1;
		`), userID).Scan(&subscriptionID)
		if err == nil {
			return subscriptionID, current.TierName, nil
		}
	}

	subscriptionID, _, tierName, _, err := r.createOrReplaceSubscription(ctx, userID, createSubscriptionRequest{TierCode: tierCode})
	if err != nil {
		return 0, "", err
	}
	return subscriptionID, tierName, nil
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

	usedUnits, err := lookupUsedUnitsInSubscriptionWindow(req.Context(), tx, r.sqlDialect, authResult.UserID, serviceItemID, startedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to process request")
		return
	}

	if usedUnits+payload.Quantity > includedUnits {
		remainingUnits := max(includedUnits-usedUnits, 0)
		writeJSON(w, http.StatusTooManyRequests, aiRequestResponse{
			Allowed:        false,
			IncludedUnits:  includedUnits,
			UsedUnits:      usedUnits,
			RemainingUnits: remainingUnits,
		})
		return
	}

	if _, err := tx.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `
		INSERT INTO als_usage_records(user_id, api_key_id, service_item_id, quantity, usage_timestamp)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP);
	`), authResult.UserID, authResult.APIKeyID, serviceItemID, payload.Quantity); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record usage")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record usage")
		return
	}

	usedAfter := usedUnits + payload.Quantity
	remainingUnits := max(includedUnits-usedAfter, 0)

	writeJSON(w, http.StatusOK, aiRequestResponse{
		Allowed:        true,
		IncludedUnits:  includedUnits,
		UsedUnits:      usedAfter,
		RemainingUnits: remainingUnits,
	})
}

func toFulfillmentJobResponse(job *fulfillment.Job) fulfillmentJobResponse {
	if job == nil {
		return fulfillmentJobResponse{}
	}

	return fulfillmentJobResponse{
		ID:             job.ID,
		EventType:      job.EventType,
		Status:         job.Status,
		UserID:         job.UserID,
		SubscriptionID: job.SubscriptionID,
		ErrorMessage:   job.ErrorMessage,
		RetryCount:     job.RetryCount,
		IdempotencyKey: job.IdempotencyKey,
	}
}

func (r *routes) loadActiveSubscription(ctx context.Context, userID int64) (subscriptionResponse, bool, error) {
	const query = `
		SELECT
			s.id,
			t.id,
			t.code,
			t.name
		FROM als_subscriptions s
		JOIN als_tiers t ON t.id = s.tier_id
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
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, query), userID).Scan(&subscriptionID, &tierID, &response.TierCode, &response.TierName)
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
		FROM als_subscriptions s
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
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, query), userID).Scan(&subscriptionID, &tierID, &startedAt)
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
		FROM als_service_items si
		LEFT JOIN als_tier_default_items tdi
			ON tdi.service_item_id = si.id
			AND tdi.tier_id = ?
		LEFT JOIN als_subscription_overrides so
			ON so.service_item_id = si.id
			AND so.subscription_id = ?
		WHERE si.id = ?
			AND (tdi.id IS NOT NULL OR so.id IS NOT NULL)
		LIMIT 1;
	`

	var includedUnits int64
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, query), tierID, subscriptionID, serviceItemID).Scan(&includedUnits)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	return includedUnits, true, nil
}

func lookupUsedUnitsInSubscriptionWindow(ctx context.Context, tx *sql.Tx, sqlDialect string, userID, serviceItemID int64, startedAt string) (int64, error) {
	const query = `
		SELECT COALESCE(SUM(quantity), 0)
		FROM als_usage_records
		WHERE user_id = ?
			AND service_item_id = ?
			AND usage_timestamp >= ?;
	`

	var usedUnits int64
	err := tx.QueryRowContext(ctx, db.Rebind(sqlDialect, query), userID, serviceItemID, startedAt).Scan(&usedUnits)
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
		FROM als_service_items si
		LEFT JOIN als_tier_default_items tdi
			ON tdi.service_item_id = si.id
			AND tdi.tier_id = ?
		LEFT JOIN als_subscription_overrides so
			ON so.service_item_id = si.id
			AND so.subscription_id = ?
		WHERE tdi.id IS NOT NULL OR so.id IS NOT NULL
		ORDER BY si.id ASC;
	`

	rows, err := r.db.QueryContext(ctx, db.Rebind(r.sqlDialect, query), tierID, subscriptionID)
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

func (r *routes) listAdminPackages(ctx context.Context) ([]adminPackageResponse, error) {
	const query = `
		SELECT
			t.id,
			t.code,
			t.name,
			t.price_micros,
			t.value_type,
			t.value_amount,
			t.description,
			t.features_json,
			t.is_enabled,
			t.created_at,
			t.updated_at,
			tgb.group_id
		FROM als_tiers t
		LEFT JOIN als_tier_group_bindings tgb ON tgb.tier_id = t.id
		ORDER BY t.id ASC, tgb.group_id ASC;
	`

	rows, err := r.db.QueryContext(ctx, db.Rebind(r.sqlDialect, query))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	packages := make([]adminPackageResponse, 0)
	packageIndex := make(map[int64]int)
	for rows.Next() {
		var (
			tierID       int64
			pkgCode      string
			pkgName      string
			priceMicros  int64
			valueType    string
			valueAmount  int64
			description  string
			featuresJSON string
			isEnabled    bool
			createdAt    string
			updatedAt    string
			groupID      sql.NullInt64
		)
		if err := rows.Scan(&tierID, &pkgCode, &pkgName, &priceMicros, &valueType, &valueAmount, &description, &featuresJSON, &isEnabled, &createdAt, &updatedAt, &groupID); err != nil {
			return nil, err
		}

		idx, found := packageIndex[tierID]
		if !found {
			idx = len(packages)
			packageIndex[tierID] = idx
			packages = append(packages, adminPackageResponse{
				Code:        pkgCode,
				Name:        pkgName,
				PriceMicros: priceMicros,
				ValueType:   valueType,
				ValueAmount: valueAmount,
				Description: description,
				Features:    parseFeaturesJSON(featuresJSON),
				IsEnabled:   isEnabled,
				GroupIDs:    []int64{},
				CreatedAt:   createdAt,
				UpdatedAt:   updatedAt,
			})
		}

		if groupID.Valid {
			packages[idx].GroupIDs = append(packages[idx].GroupIDs, groupID.Int64)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return packages, nil
}

func parseFeaturesJSON(raw string) []string {
	if raw == "" || raw == "[]" {
		return []string{}
	}
	var features []string
	if err := json.Unmarshal([]byte(raw), &features); err != nil {
		return []string{}
	}
	return features
}

func (r *routes) loadAdminPackageByCode(ctx context.Context, packageCode string) (adminPackageResponse, error) {
	packages, err := r.listAdminPackages(ctx)
	if err != nil {
		return adminPackageResponse{}, err
	}
	for _, pkg := range packages {
		if pkg.Code == packageCode {
			return pkg, nil
		}
	}
	return adminPackageResponse{}, sql.ErrNoRows
}

func (r *routes) loadAuthorizedGroupIDSet(ctx context.Context, userID int64) (map[int64]struct{}, error) {
	_, tierID, _, found, err := r.lookupActiveSubscriptionContext(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !found {
		return map[int64]struct{}{}, nil
	}

	rows, err := r.db.QueryContext(ctx, db.Rebind(r.sqlDialect, `SELECT group_id FROM als_tier_group_bindings WHERE tier_id = ? ORDER BY group_id ASC;`), tierID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]struct{})
	for rows.Next() {
		var groupID int64
		if err := rows.Scan(&groupID); err != nil {
			return nil, err
		}
		if groupID > 0 {
			result[groupID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *routes) loadAuthorizedVisibleGroupIDs(req *http.Request, authorizedGroupIDs map[int64]struct{}) (map[int64]struct{}, error) {
	if len(authorizedGroupIDs) == 0 {
		return map[int64]struct{}{}, nil
	}
	payload, err := r.loadUpstreamJSONPayload(req, "/api/v1/groups/available")
	if err != nil {
		return nil, err
	}
	return extractAuthorizedGroupIDs(payload, authorizedGroupIDs), nil
}

func (r *routes) loadUpstreamJSONPayload(req *http.Request, upstreamPath string) (any, error) {
	forwarded := req.Clone(req.Context())
	forwarded.Header = cloneHeaders(req.Header)
	forwarded.Header.Del("X-User-Id")
	if err := r.replaceAuthorizationWithStoredUpstreamToken(req.Context(), forwarded.Header); err != nil {
		return nil, err
	}

	resp, err := r.proxyClient.Do(req.Context(), forwarded, upstreamPath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload any
	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (r *routes) filteredProxyJSONResponse(w http.ResponseWriter, req *http.Request, upstreamPath string, filterFn func(any) (any, error)) (any, int, http.Header, bool, error) {
	forwarded := req.Clone(req.Context())
	forwarded.Header = cloneHeaders(req.Header)
	forwarded.Header.Del("X-User-Id")
	if err := r.replaceAuthorizationWithStoredUpstreamToken(req.Context(), forwarded.Header); err != nil {
		if errors.Is(err, sub2apiauth.ErrTokenNotFound) {
			writeError(w, http.StatusUnauthorized, "upstream session unavailable")
			return nil, 0, nil, false, nil
		}
		return nil, 0, nil, false, err
	}

	resp, err := r.proxyClient.Do(req.Context(), forwarded, upstreamPath)
	if err != nil {
		return nil, 0, nil, false, err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if copyErr := proxy.CopyResponse(w, resp); copyErr != nil {
			log.Printf("proxy filtered response copy failed for %s: %v", upstreamPath, copyErr)
			return nil, 0, nil, false, nil
		}
		return nil, 0, nil, false, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, nil, false, err
	}

	var payload any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, 0, nil, false, err
	}

	filteredPayload, err := filterFn(payload)
	if err != nil {
		return nil, 0, nil, false, err
	}

	return filteredPayload, resp.StatusCode, resp.Header.Clone(), true, nil
}

func writeForwardedJSON(w http.ResponseWriter, statusCode int, headers http.Header, payload any) {
	for name, values := range headers {
		canonical := http.CanonicalHeaderKey(name)
		switch canonical {
		case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade", "Content-Length":
			continue
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func filterGroupListPayloadByID(payload any, authorizedGroupIDs map[int64]struct{}) (any, error) {
	if root, ok := payload.(map[string]any); ok {
		cloned := cloneMap(root)
		if groups, ok := root["data"].([]any); ok {
			cloned["data"] = filterGroupItemsByID(groups, authorizedGroupIDs)
			return cloned, nil
		}
	}
	if groups, ok := payload.([]any); ok {
		return filterGroupItemsByID(groups, authorizedGroupIDs), nil
	}
	return payload, nil
}

func filterAPIKeyListPayload(payload any, authorizedGroupIDs map[int64]struct{}) (any, error) {
	if root, ok := payload.(map[string]any); ok {
		clonedRoot := cloneMap(root)
		if dataMap, ok := root["data"].(map[string]any); ok {
			clonedData := cloneMap(dataMap)
			if items, ok := dataMap["data"].([]any); ok {
				filtered := filterAPIKeyItems(items, authorizedGroupIDs)
				clonedData["data"] = filtered
				clonedData["total"] = len(filtered)
				clonedRoot["data"] = clonedData
				return clonedRoot, nil
			}
			if items, ok := dataMap["items"].([]any); ok {
				filtered := filterAPIKeyItems(items, authorizedGroupIDs)
				clonedData["items"] = filtered
				clonedData["total"] = len(filtered)
				clonedRoot["data"] = clonedData
				if pagination, ok := root["pagination"].(map[string]any); ok {
					clonedPagination := cloneMap(pagination)
					clonedPagination["total"] = len(filtered)
					clonedRoot["pagination"] = clonedPagination
				}
				return clonedRoot, nil
			}
		}
		if items, ok := root["data"].([]any); ok {
			clonedRoot["data"] = filterAPIKeyItems(items, authorizedGroupIDs)
			return clonedRoot, nil
		}
	}
	if items, ok := payload.([]any); ok {
		return filterAPIKeyItems(items, authorizedGroupIDs), nil
	}
	return payload, nil
}

func isAPIKeyPayloadAuthorized(payload any, authorizedGroupIDs map[int64]struct{}) (bool, error) {
	item, ok := unwrapSingleAPIKeyPayload(payload)
	if !ok {
		return false, errors.New("api key payload shape is not supported")
	}
	groupID, ok := extractGroupID(item)
	if !ok {
		return false, nil
	}
	_, allowed := authorizedGroupIDs[groupID]
	return allowed, nil
}

func filterGroupItemsByID(groups []any, authorizedGroupIDs map[int64]struct{}) []any {
	filtered := make([]any, 0, len(groups))
	for _, raw := range groups {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		groupID, ok := asInt64(item["id"])
		if !ok {
			continue
		}
		if _, allowed := authorizedGroupIDs[groupID]; allowed {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func extractAuthorizedGroupIDs(payload any, authorizedGroupIDs map[int64]struct{}) map[int64]struct{} {
	result := make(map[int64]struct{})
	filtered, _ := filterGroupListPayloadByID(payload, authorizedGroupIDs)
	for _, item := range extractGroupItems(filtered) {
		if groupID, ok := asInt64(item["id"]); ok {
			result[groupID] = struct{}{}
		}
	}
	return result
}

func extractGroupItems(payload any) []map[string]any {
	if root, ok := payload.(map[string]any); ok {
		if groups, ok := root["data"].([]any); ok {
			return toObjectSlice(groups)
		}
	}
	if groups, ok := payload.([]any); ok {
		return toObjectSlice(groups)
	}
	return nil
}

func filterAPIKeyItems(items []any, authorizedGroupIDs map[int64]struct{}) []any {
	filtered := make([]any, 0, len(items))
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		groupID, ok := extractGroupID(item)
		if !ok {
			continue
		}
		if _, allowed := authorizedGroupIDs[groupID]; allowed {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func unwrapSingleAPIKeyPayload(payload any) (map[string]any, bool) {
	if root, ok := payload.(map[string]any); ok {
		if dataMap, ok := root["data"].(map[string]any); ok {
			return dataMap, true
		}
		return root, true
	}
	return nil, false
}

func extractGroupID(item map[string]any) (int64, bool) {
	if groupID, ok := asInt64(item["group_id"]); ok {
		return groupID, true
	}
	if groupMap, ok := item["group"].(map[string]any); ok {
		return asInt64(groupMap["id"])
	}
	return 0, false
}

func toObjectSlice(items []any) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		if item, ok := raw.(map[string]any); ok {
			result = append(result, item)
		}
	}
	return result
}

func cloneMap(source map[string]any) map[string]any {
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func asInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed, true
		}
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func (r *routes) replaceTierGroupBindingsTx(ctx context.Context, tx *sql.Tx, tierID int64, groupIDs []int64, now string) error {
	if _, err := tx.ExecContext(ctx, db.Rebind(r.sqlDialect, `DELETE FROM als_tier_group_bindings WHERE tier_id = ?;`), tierID); err != nil {
		return err
	}
	for _, groupID := range groupIDs {
		if _, err := tx.ExecContext(ctx, db.Rebind(r.sqlDialect, `
			INSERT INTO als_tier_group_bindings(tier_id, group_id, created_at, updated_at)
			VALUES (?, ?, ?, ?);
		`), tierID, groupID, now, now); err != nil {
			return err
		}
	}
	return nil
}

func (r *routes) lookupTier(ctx context.Context, tierCode string) (int64, string, error) {
	var (
		tierID   int64
		tierName string
	)
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `SELECT id, name FROM als_tiers WHERE code = ?;`), tierCode).Scan(&tierID, &tierName)
	if err != nil {
		return 0, "", err
	}
	return tierID, tierName, nil
}

func (r *routes) lookupTierID(ctx context.Context, tierCode string) (int64, error) {
	var tierID int64
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `SELECT id FROM als_tiers WHERE code = ?;`), tierCode).Scan(&tierID)
	if err != nil {
		return 0, err
	}
	return tierID, nil
}

func (r *routes) lookupServiceItemID(ctx context.Context, serviceItemCode string) (int64, error) {
	var serviceItemID int64
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `SELECT id FROM als_service_items WHERE code = ?;`), serviceItemCode).Scan(&serviceItemID)
	if err != nil {
		return 0, err
	}
	return serviceItemID, nil
}

func lookupServiceItemID(ctx context.Context, tx *sql.Tx, sqlDialect string, serviceItemCode string) (int64, error) {
	var serviceItemID int64
	err := tx.QueryRowContext(ctx, db.Rebind(sqlDialect, `SELECT id FROM als_service_items WHERE code = ?;`), serviceItemCode).Scan(&serviceItemID)
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

func parsePositiveInt64(raw string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0, errors.New("value must be a positive integer")
	}
	return value, nil
}

func microsToCurrencyMinor(micros int64) (int64, error) {
	if micros <= 0 {
		return 0, errors.New("package price must be positive")
	}
	return int64((micros + 5000) / 10000), nil
}

func microsToFloatCurrency(micros int64) float64 {
	if micros <= 0 {
		return 0
	}
	return float64(micros) / 1_000_000
}

func buildRedeemCode(orderID, fallback string, groupID int64) string {
	base := strings.TrimSpace(orderID)
	if base == "" {
		base = strings.TrimSpace(fallback)
	}
	if base == "" {
		base = "als-payment"
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d", base, groupID)))
	return hex.EncodeToString(sum[:16])
}

func normalizeAdminPackageRequest(payload adminPackageRequest, requireCode bool) (adminPackageRequest, error) {
	payload.Code = strings.TrimSpace(payload.Code)
	payload.Name = strings.TrimSpace(payload.Name)
	payload.ValueType = strings.TrimSpace(payload.ValueType)
	payload.Description = strings.TrimSpace(payload.Description)
	payload.FeaturesJSON = strings.TrimSpace(payload.FeaturesJSON)

	if requireCode && payload.Code == "" {
		return adminPackageRequest{}, errors.New("code is required")
	}
	if payload.Name == "" {
		return adminPackageRequest{}, errors.New("name is required")
	}
	if len(payload.GroupIDs) == 0 {
		return adminPackageRequest{}, errors.New("group_ids is required")
	}
	if payload.PriceMicros < 0 {
		return adminPackageRequest{}, errors.New("price_micros must be >= 0")
	}

	switch payload.ValueType {
	case "", "days", "balance":
		// valid
	default:
		return adminPackageRequest{}, errors.New("value_type must be empty, 'days', or 'balance'")
	}
	if payload.ValueType != "" && payload.ValueAmount <= 0 {
		return adminPackageRequest{}, errors.New("value_amount must be > 0 when value_type is set")
	}

	if payload.FeaturesJSON != "" && payload.FeaturesJSON != "[]" {
		if !json.Valid([]byte(payload.FeaturesJSON)) {
			return adminPackageRequest{}, errors.New("features_json must be valid JSON")
		}
		var arr []string
		if err := json.Unmarshal([]byte(payload.FeaturesJSON), &arr); err != nil {
			return adminPackageRequest{}, errors.New("features_json must be a JSON array of strings")
		}
	} else {
		payload.FeaturesJSON = "[]"
	}

	normalizedGroupIDs := make([]int64, 0, len(payload.GroupIDs))
	seen := make(map[int64]struct{}, len(payload.GroupIDs))
	for _, rawID := range payload.GroupIDs {
		if rawID <= 0 {
			return adminPackageRequest{}, errors.New("group_ids must be positive integers")
		}
		if _, exists := seen[rawID]; exists {
			continue
		}
		seen[rawID] = struct{}{}
		normalizedGroupIDs = append(normalizedGroupIDs, rawID)
	}
	if len(normalizedGroupIDs) == 0 {
		return adminPackageRequest{}, errors.New("group_ids is required")
	}
	sort.Slice(normalizedGroupIDs, func(i, j int) bool { return normalizedGroupIDs[i] < normalizedGroupIDs[j] })
	payload.GroupIDs = normalizedGroupIDs
	return payload, nil
}

func validateArticleSlug(slug string) error {
	v := strings.TrimSpace(slug)
	if v == "" {
		return errors.New("slug is required")
	}
	if len(v) > maxArticleSlugLength {
		return errors.New("slug must be 128 characters or fewer")
	}
	if !articleSlugPattern.MatchString(v) {
		return errors.New("slug must match ^[a-z0-9]+(?:-[a-z0-9]+)*$")
	}
	return nil
}

func normalizeAdminArticleStatus(raw string, defaultDraft bool) (string, error) {
	status := strings.TrimSpace(strings.ToLower(raw))
	if status == "" {
		if defaultDraft {
			return adminArticleStatusDraft, nil
		}
		return "", nil
	}
	if status != adminArticleStatusDraft && status != adminArticleStatusPublished {
		return "", errors.New("invalid article status")
	}
	return status, nil
}

func isValidAdminStatusTransition(from, to string) bool {
	current := strings.TrimSpace(strings.ToLower(from))
	next := strings.TrimSpace(strings.ToLower(to))

	if next == "" {
		return true
	}
	if current == "" {
		current = adminArticleStatusDraft
	}

	switch current {
	case adminArticleStatusDraft:
		return next == adminArticleStatusDraft || next == adminArticleStatusPublished
	case adminArticleStatusPublished:
		return next == adminArticleStatusDraft || next == adminArticleStatusPublished
	default:
		return false
	}
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func toAdminArticleDTO(item model.Article) adminArticleDTO {
	var publishedAt *string
	if item.PublishedAt != nil {
		formatted := item.PublishedAt.UTC().Format(time.RFC3339)
		publishedAt = &formatted
	}

	return adminArticleDTO{
		ID:              item.ID,
		LegacyID:        item.LegacyID,
		Slug:            item.Slug,
		Title:           item.Title,
		Excerpt:         item.Excerpt,
		CoverImageURL:   item.CoverImageURL,
		Tag:             item.Tag,
		ReadTime:        item.ReadTime,
		AuthorName:      item.AuthorName,
		AuthorAvatarURL: item.AuthorAvatarURL,
		AuthorIcon:      item.AuthorIcon,
		MDXBody:         item.MDXBody,
		Status:          item.Status,
		PublishedAt:     publishedAt,
		CreatedByUserID: item.CreatedByUserID,
		UpdatedByUserID: item.UpdatedByUserID,
		CreatedAt:       item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (r *routes) findAdminArticleBySlug(ctx context.Context, slug string) (*model.Article, error) {
	als_articles, err := r.articleSvc.ListArticles(ctx, article.ListArticlesFilters{})
	if err != nil {
		return nil, err
	}
	for idx := range als_articles {
		if als_articles[idx].Slug == slug {
			item := als_articles[idx]
			return &item, nil
		}
	}
	return nil, article.ErrArticleNotFound
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
		FROM als_unit_prices
		WHERE service_item_id = ?
			AND effective_to IS NULL
			AND (tier_id = ? OR tier_id IS NULL)
		ORDER BY
			CASE WHEN tier_id = ? THEN 0 ELSE 1 END,
			effective_from DESC
		LIMIT 1;
	`

	var result unitPriceRow
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, query), serviceItemID, tierID, tierID).Scan(&result.PricePerUnitMicros, &result.Currency)
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

func cloneHeaders(headers http.Header) http.Header {
	cloned := make(http.Header, len(headers))
	for key, values := range headers {
		copiedValues := make([]string, len(values))
		copy(copiedValues, values)
		cloned[key] = copiedValues
	}
	return cloned
}
