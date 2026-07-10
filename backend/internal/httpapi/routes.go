package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ai-api-portal/backend/internal/apikey"
	"ai-api-portal/backend/internal/article"
	"ai-api-portal/backend/internal/auth"
	"ai-api-portal/backend/internal/configcenter"
	"ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/doc"
	"ai-api-portal/backend/internal/download"
	"ai-api-portal/backend/internal/fulfillment"
	"ai-api-portal/backend/internal/model"
	"ai-api-portal/backend/internal/proxy"
	"ai-api-portal/backend/internal/scanlogin"
	"ai-api-portal/backend/internal/servicedirection"
	portalstripe "ai-api-portal/backend/internal/stripe"
	"ai-api-portal/backend/internal/sub2api"
	"ai-api-portal/backend/internal/sub2apiauth"
	"ai-api-portal/backend/internal/user"
)

type routes struct {
	db                   *sql.DB
	sqlDialect           string
	apiKey               *apikey.Service
	articleSvc           *article.Service
	docSvc               *doc.Service
	configCenterSvc      *configcenter.Service
	downloadSvc          *download.Service
	serviceDirectionSvc  *servicedirection.Service
	fulfillmentSvc       *fulfillment.Service
	userSvc              *user.Service
	sub2api              *sub2api.Gateway
	proxyClient          *proxy.Client
	stripeClient         *portalstripe.Client
	userUsageCache       UserUsageCache
	adminBootstrapSecret string
	scanLogin            *scanlogin.Service
	// refreshArbiter serializes refreshes per user so concurrent/multi-device
	// refreshes never reach sub2api twice with the same token (which would trip
	// its refresh-token replay detection and revoke the whole family). The map
	// is lazily initialised and never shrinks; keyed by user id.
	refreshMu     sync.Mutex
	refreshUserMu map[int64]*sync.Mutex
}

type RoutesOptions struct {
	UserService          *user.Service
	ProxyClient          *proxy.Client
	StripeClient         *portalstripe.Client
	UserUsageCache       UserUsageCache
	AdminBootstrapSecret string
	SQLDialect           string
}

type UserUsageCache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
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

type adminQuickCreateUserRequest struct {
	Email string `json:"email"`
}

type adminQuickCreateUserResponse struct {
	ID                   int64  `json:"id"`
	DistributorBindingID int64  `json:"distributor_binding_id,omitempty"`
	Email                string `json:"email"`
	Name                 string `json:"name"`
	Password             string `json:"password"`
	CreatedAt            string `json:"created_at"`
}

type adminUpdateUserRoleRequest struct {
	UserID int64  `json:"user_id,omitempty"`
	Email  string `json:"email,omitempty"`
	Role   string `json:"role"`
}

type adminUpdateUserRoleResponse struct {
	ID            int64  `json:"id"`
	Sub2APIUserID *int64 `json:"sub2api_user_id,omitempty"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Role          string `json:"role"`
	UpdatedAt     string `json:"updated_at"`
}

type adminAssignPackageRequest struct {
	UserID   int64  `json:"user_id"`
	Email    string `json:"email,omitempty"`
	TierCode string `json:"tier_code"`
	Password string `json:"password,omitempty"`
}

type adminAssignPackageResponse struct {
	PaymentEventID string                  `json:"payment_event_id"`
	TierCode       string                  `json:"tier_code"`
	FulfillmentJob *fulfillmentJobResponse `json:"fulfillment_job,omitempty"`
}

type distributorAssignmentStatsTotalsResponse struct {
	AssignmentCount  int64 `json:"assignment_count"`
	UniqueUserCount  int64 `json:"unique_user_count"`
	DistributorCount int64 `json:"distributor_count,omitempty"`
	TotalPriceMicros int64 `json:"total_price_micros"`
}

type distributorAssignmentDailyStatsResponse struct {
	Date             string `json:"date"`
	AssignmentCount  int64  `json:"assignment_count"`
	TotalPriceMicros int64  `json:"total_price_micros"`
}

type distributorAssignmentPackageStatsResponse struct {
	TierCode         string `json:"tier_code"`
	PackageName      string `json:"package_name"`
	AssignmentCount  int64  `json:"assignment_count"`
	TotalPriceMicros int64  `json:"total_price_micros"`
	LatestAssignedAt string `json:"latest_assigned_at,omitempty"`
}

type distributorAssignmentUserStatsResponse struct {
	UserID           int64  `json:"user_id"`
	Email            string `json:"email"`
	Name             string `json:"name"`
	AssignmentCount  int64  `json:"assignment_count"`
	TotalPriceMicros int64  `json:"total_price_micros"`
	LatestAssignedAt string `json:"latest_assigned_at,omitempty"`
}

type distributorAssignmentDistributorStatsResponse struct {
	DistributorUserID int64  `json:"distributor_user_id"`
	DistributorEmail  string `json:"distributor_email"`
	DistributorName   string `json:"distributor_name"`
	AssignmentCount   int64  `json:"assignment_count"`
	UniqueUserCount   int64  `json:"unique_user_count"`
	TotalPriceMicros  int64  `json:"total_price_micros"`
	LatestAssignedAt  string `json:"latest_assigned_at,omitempty"`
}

type distributorAssignmentStatsResponse struct {
	Totals       distributorAssignmentStatsTotalsResponse        `json:"totals"`
	Daily        []distributorAssignmentDailyStatsResponse       `json:"daily"`
	Packages     []distributorAssignmentPackageStatsResponse     `json:"packages"`
	Users        []distributorAssignmentUserStatsResponse        `json:"users"`
	Distributors []distributorAssignmentDistributorStatsResponse `json:"distributors,omitempty"`
}

type adminBindDistributorUserRequest struct {
	DistributorUserID int64  `json:"distributor_user_id,omitempty"`
	DistributorEmail  string `json:"distributor_email,omitempty"`
	UserID            int64  `json:"user_id,omitempty"`
	Email             string `json:"email,omitempty"`
	Source            string `json:"source,omitempty"`
}

type distributorUserBindingResponse struct {
	ID                int64  `json:"id"`
	DistributorUserID int64  `json:"distributor_user_id"`
	UserID            int64  `json:"user_id"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	Source            string `json:"source"`
	CreatedAt         string `json:"created_at"`
}

type distributorInvitationResponse struct {
	ID                int64  `json:"id"`
	DistributorUserID int64  `json:"distributor_user_id"`
	DistributorEmail  string `json:"distributor_email,omitempty"`
	DistributorName   string `json:"distributor_name,omitempty"`
	UserID            int64  `json:"user_id"`
	UpstreamUserID    *int64 `json:"upstream_user_id,omitempty"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	Source            string `json:"source"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at,omitempty"`
}

type distributorUserSummaryResponse struct {
	UserID             int64  `json:"user_id"`
	UpstreamUserID     *int64 `json:"upstream_user_id,omitempty"`
	Email              string `json:"email"`
	Name               string `json:"name"`
	PackageCode        string `json:"package_code,omitempty"`
	PackageName        string `json:"package_name,omitempty"`
	SubscriptionStatus string `json:"subscription_status,omitempty"`
	TotalTokens        int64  `json:"total_tokens"`
	ActiveDays         int64  `json:"active_days"`
	ActualCostMicros   int64  `json:"actual_cost_micros"`
	LastActiveDate     string `json:"last_active_date,omitempty"`
	UsageSyncedAt      string `json:"usage_synced_at,omitempty"`
	UsageSource        string `json:"usage_source,omitempty"`
	UsageStale         bool   `json:"usage_stale,omitempty"`
	UsageUnavailable   bool   `json:"usage_unavailable,omitempty"`
}

type paginationResponse struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

type listPaginationRequest struct {
	Page    int
	PerPage int
	Offset  int
}

type listDistributorUsersResponse struct {
	Users      []distributorUserSummaryResponse `json:"users"`
	Pagination paginationResponse               `json:"pagination"`
}

type listDistributorInvitationsResponse struct {
	Invitations []distributorInvitationResponse `json:"invitations"`
	Pagination  paginationResponse              `json:"pagination"`
}

type createPackageCheckoutSessionRequest struct {
	TierCode     string `json:"tier_code"`
	AmountMicros int64  `json:"amount_micros,omitempty"`
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
	BillingType      string `json:"billing_type,omitempty"`
}

type adminAvailableGroupsResponse struct {
	Groups []adminGroupResponse `json:"groups"`
}

type adminPackageRequest struct {
	Code           string  `json:"code,omitempty"`
	Name           string  `json:"name"`
	Level          string  `json:"level,omitempty"`
	GroupIDs       []int64 `json:"group_ids"`
	PriceMicros    int64   `json:"price_micros"`
	ValueType      string  `json:"value_type"`
	ValueAmount    int64   `json:"value_amount"`
	Rate           float64 `json:"rate,omitempty"`
	MinTopupMicros int64   `json:"min_topup_micros,omitempty"`
	MaxTopupMicros int64   `json:"max_topup_micros,omitempty"`
	Description    string  `json:"description"`
	FeaturesJSON   string  `json:"features_json"`
	IsEnabled      *bool   `json:"is_enabled,omitempty"`
	IsVisible      *bool   `json:"is_visible,omitempty"`
	IsPublished    *bool   `json:"is_published,omitempty"`
}

type adminPackageResponse struct {
	Code           string   `json:"code"`
	Name           string   `json:"name"`
	Level          string   `json:"level"`
	GroupIDs       []int64  `json:"group_ids"`
	PriceMicros    int64    `json:"price_micros"`
	ValueType      string   `json:"value_type"`
	ValueAmount    int64    `json:"value_amount"`
	Rate           float64  `json:"rate"`
	MinTopupMicros int64    `json:"min_topup_micros"`
	MaxTopupMicros int64    `json:"max_topup_micros"`
	Description    string   `json:"description"`
	Features       []string `json:"features"`
	IsEnabled      bool     `json:"is_enabled"`
	IsVisible      bool     `json:"is_visible"`
	IsPublished    bool     `json:"is_published"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

type listAdminPackagesResponse struct {
	Packages []adminPackageResponse `json:"packages"`
}

type publicPackageResponse struct {
	Code           string   `json:"code"`
	Name           string   `json:"name"`
	PriceMicros    int64    `json:"price_micros"`
	ValueType      string   `json:"value_type"`
	ValueAmount    int64    `json:"value_amount"`
	Rate           float64  `json:"rate"`
	MinTopupMicros int64    `json:"min_topup_micros"`
	MaxTopupMicros int64    `json:"max_topup_micros"`
	Description    string   `json:"description"`
	Features       []string `json:"features"`
	IsPublished    bool     `json:"is_published"`
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
	adminDocSlugLength          = 128
	packageLevelAdmin           = "admin"
	packageLevelDistributor     = "distributor"
)

// ── Doc DTO DTO────────────────────────────────────

type adminDocCategoryDTO struct {
	ID          int64   `json:"id"`
	Slug        string  `json:"slug"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Icon        *string `json:"icon,omitempty"`
	SortOrder   int64   `json:"sort_order"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type adminDocCategoryListResponse struct {
	Categories []adminDocCategoryDTO `json:"categories"`
}

type adminDocCategoryDetailResponse struct {
	Category adminDocCategoryDTO `json:"category"`
}

type adminDocPageDTO struct {
	ID         int64  `json:"id"`
	Slug       string `json:"slug"`
	Title      string `json:"title"`
	CategoryID int64  `json:"category_id"`
	MDXBody    string `json:"mdx_body"`
	SortOrder  int64  `json:"sort_order"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type adminDocPageListResponse struct {
	Pages []adminDocPageDTO `json:"pages"`
}

type adminDocPageDetailResponse struct {
	Page adminDocPageDTO `json:"page"`
}

type publicDocCategoryWithPagesDTO struct {
	Slug        string                 `json:"slug"`
	Title       string                 `json:"title"`
	Description *string                `json:"description,omitempty"`
	Pages       []publicDocPageSummary `json:"pages"`
}

type publicDocPageSummary struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	MDXBody string `json:"mdx_body"`
}

type publicDocPageDetailResponse struct {
	Page publicDocPageSummary `json:"page"`
}

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
		userSvc = user.NewServiceWithOptions(database, user.ServiceOptions{SQLDialect: strings.TrimSpace(opts.SQLDialect)})
	}
	// 共享同一个 sub2api Gateway：既作为 upstream passthrough 入口，
	// 也作为扫码登录的 refresh_token 解析器（authorized 时把 sub2api refresh_token 连同 st_ 下发）。
	sub2apiGateway := sub2api.NewGateway(opts.ProxyClient, sub2apiauth.NewServiceWithDialect(database, strings.TrimSpace(opts.SQLDialect)))
	r := &routes{
		db:                   database,
		sqlDialect:           strings.TrimSpace(opts.SQLDialect),
		apiKey:               apikey.NewService(database),
		articleSvc:           article.NewServiceWithDialect(database, strings.TrimSpace(opts.SQLDialect)),
		docSvc:               doc.NewService(database, strings.TrimSpace(opts.SQLDialect)),
		configCenterSvc:      configcenter.NewService(database, strings.TrimSpace(opts.SQLDialect)),
		downloadSvc:          download.NewService(database, strings.TrimSpace(opts.SQLDialect)),
		serviceDirectionSvc:  servicedirection.NewService(database, strings.TrimSpace(opts.SQLDialect)),
		fulfillmentSvc:       fulfillment.NewServiceWithDialect(database, strings.TrimSpace(opts.SQLDialect)),
		userSvc:              userSvc,
		sub2api:              sub2apiGateway,
		proxyClient:          opts.ProxyClient,
		stripeClient:         opts.StripeClient,
		adminBootstrapSecret: strings.TrimSpace(opts.AdminBootstrapSecret),
		userUsageCache:       opts.UserUsageCache,
		scanLogin:            scanlogin.NewService(database, scanlogin.Options{Dialect: strings.TrimSpace(opts.SQLDialect), Minter: userSvc, RefreshTokenResolver: sub2apiGateway}),
	}
	authenticated := auth.RequireUserWithDialect(database, r.sqlDialect)

	// 扫码登录（本地能力，非 upstream passthrough）
	mux.HandleFunc("POST /auth/scan/init", r.handleScanInit)
	mux.HandleFunc("GET /auth/scan/status", r.handleScanStatus)
	mux.Handle("POST /auth/scan/scan", r.requireUserForScan(http.HandlerFunc(r.handleScanScan)))
	mux.Handle("POST /auth/scan/confirm", r.requireUserForScan(http.HandlerFunc(r.handleScanConfirm)))
	mux.Handle("POST /auth/scan/deny", r.requireUserForScan(http.HandlerFunc(r.handleScanDeny)))

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

		// alianggate app client compatibility routes
		mux.HandleFunc("POST /api/auth/login", r.handleAuthLoginPassthrough)
		mux.HandleFunc("POST /api/auth/refresh", r.handleAuthRefreshPassthrough)
		mux.HandleFunc("POST /api/auth/logout", r.handleAuthLogoutPassthrough)
		mux.HandleFunc("GET /api/auth/me", r.handleAuthMePassthrough)
		// Mobile Me page routes (aliangVibeCodingPhone) — /api/ prefix aliases
		mux.HandleFunc("GET /api/dashboard/account", r.handleDashboardAccountPassthrough)
		mux.HandleFunc("GET /api/dashboard/usage", r.handleDashboardUsagePassthrough)
		mux.HandleFunc("GET /api/subscriptions/active", r.handleSubscriptionsActivePassthrough)
		mux.HandleFunc("GET /api/subscriptions/summary", r.handleSubscriptionsSummaryPassthrough)

		// /api/v1/auth/* — alianggate compat paths
		mux.HandleFunc("POST /api/v1/auth/register", r.handleAuthRegisterPassthrough)
		mux.HandleFunc("POST /api/v1/auth/login", r.handleAuthLoginPassthrough)
		mux.HandleFunc("POST /api/v1/auth/refresh", r.handleAuthRefreshPassthrough)
		mux.HandleFunc("POST /api/v1/auth/logout", r.handleAuthLogoutPassthrough)
		mux.HandleFunc("GET /api/v1/auth/me", r.handleAuthMePassthrough)

		// alianggate user-center paths (bearer passthrough; no local-session gate)
		mux.HandleFunc("GET /api/v1/user/profile", r.handleUserProfilePassthrough)
		mux.HandleFunc("PUT /api/v1/user/profile", r.handleUserProfileUpdatePassthrough)
		mux.HandleFunc("GET /api/v1/user/subscriptions/summary", r.handleUserSubSummaryPassthrough)
		mux.HandleFunc("GET /api/v1/user/subscriptions/progress", r.handleUserSubProgressPassthrough)
		mux.Handle("GET /api/v1/user/groups/available", authenticated(http.HandlerFunc(r.handleUserGroupsAvailablePassthrough)))
		mux.Handle("GET /api/v1/user/api_keys", authenticated(http.HandlerFunc(r.handleUserAPIKeysPassthrough)))
		mux.HandleFunc("POST /api/v1/user/code/redeem", r.handleUserCodeRedeemPassthrough)

		// upstream-compatible passthrough routes (alianggate native paths)
		mux.HandleFunc("PUT /api/v1/user", r.handleUpstreamPassthrough("/api/v1/user"))
		mux.HandleFunc("GET /api/v1/subscriptions/summary", r.handleUpstreamPassthrough("/api/v1/subscriptions/summary"))
		mux.HandleFunc("GET /api/v1/subscriptions/progress", r.handleUpstreamPassthrough("/api/v1/subscriptions/progress"))
		mux.HandleFunc("GET /api/v1/keys", r.handleUpstreamPassthrough("/api/v1/keys"))
		mux.HandleFunc("GET /api/v1/groups/available", r.handleUpstreamPassthrough("/api/v1/groups/available"))
		mux.HandleFunc("POST /api/v1/redeem", r.handleUpstreamPassthrough("/api/v1/redeem"))

		// alianggate dashboard / usage paths (native /api/v1/*, same upstream as /dashboard/*)
		mux.HandleFunc("GET /api/v1/usage/dashboard/stats", r.handleDashboardHomePassthrough)
		mux.HandleFunc("GET /api/v1/usage/dashboard/trend", r.handleDashboardDetailsPassthrough)
		mux.HandleFunc("GET /api/v1/usage/dashboard/models", r.handleDashboardModelsPassthrough)
		mux.HandleFunc("GET /api/v1/usage", r.handleDashboardUsagePassthrough)
		mux.HandleFunc("GET /api/v1/usage/stats", r.handleUsageStatsPassthrough)
		mux.HandleFunc("GET /api/v1/admin/ops/dashboard/snapshot-v2", r.handleOpsDashboardSnapshotPassthrough)

		mux.Handle("POST /api/user/auth/new/activate", authenticated(http.HandlerFunc(r.handleUpstreamPassthrough("/api/user/auth/new/activate"))))
		mux.Handle("GET /api/user/auth/info/plan/info", authenticated(http.HandlerFunc(r.handleUpstreamPassthrough("/api/user/auth/info/plan/info"))))
		mux.Handle("GET /api/production/prod/sui/user/sui/inbounds", authenticated(http.HandlerFunc(r.handleUpstreamPassthrough("/api/production/prod/sui/user/sui/inbounds"))))

		mux.HandleFunc("GET /api/public/downloads/check", r.handlePublicVersionCheck)
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
	mux.HandleFunc("GET /public/payment-config", http.HandlerFunc(r.handlePublicGetPaymentConfig))
	mux.Handle("POST /subscription", authenticated(http.HandlerFunc(r.handleCreateSubscription)))
	mux.Handle("POST /checkout/package", authenticated(http.HandlerFunc(r.handleCreatePackageCheckoutSession)))
	mux.Handle("GET /admin/unit-prices", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListUnitPrices))))
	mux.Handle("PUT /admin/unit-prices", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminSetUnitPrice))))
	mux.Handle("DELETE /admin/unit-prices", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminDeactivateUnitPrice))))
	mux.Handle("POST /admin/fulfillment/payment-success", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminPaymentSuccess))))
	mux.Handle("POST /admin/users/quick-create", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminQuickCreateUser))))
	mux.Handle("PUT /admin/users/role", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminUpdateUserRole))))
	mux.Handle("GET /admin/distributor/users", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListDistributorInvitations))))
	mux.Handle("POST /admin/distributor/users", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminBindDistributorUser))))
	mux.Handle("GET /admin/distributor/stats", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminDistributorAssignmentStats))))
	mux.Handle("POST /admin/users/assign-package", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminAssignPackage))))
	mux.Handle("POST /distributor/users/quick-create", authenticated(auth.RequireDistributor(http.HandlerFunc(r.handleDistributorQuickCreateUser))))
	mux.Handle("GET /distributor/users", authenticated(auth.RequireDistributor(http.HandlerFunc(r.handleDistributorListUsers))))
	mux.Handle("GET /distributor/invitations", authenticated(auth.RequireDistributor(http.HandlerFunc(r.handleDistributorListInvitations))))
	mux.Handle("GET /distributor/packages", authenticated(auth.RequireDistributor(http.HandlerFunc(r.handleDistributorListPackages))))
	mux.Handle("GET /distributor/stats", authenticated(auth.RequireDistributor(http.HandlerFunc(r.handleDistributorAssignmentStats))))
	mux.Handle("POST /distributor/assign-package", authenticated(auth.RequireDistributor(http.HandlerFunc(r.handleAdminAssignPackage))))
	mux.Handle("GET /admin/fulfillment/jobs/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminGetFulfillmentJob))))
	mux.Handle("POST /admin/fulfillment/jobs/{id}/replay", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminReplayFulfillmentJob))))
	mux.Handle("GET /admin/groups/available", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListAvailableGroups))))
	mux.Handle("GET /admin/packages", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListPackages))))
	mux.Handle("POST /admin/packages", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminCreatePackage))))
	mux.Handle("GET /admin/packages/{code}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminGetPackage))))
	mux.Handle("PUT /admin/packages/{code}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminUpdatePackage))))
	mux.Handle("DELETE /admin/packages/{code}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminDeletePackage))))
	mux.Handle("GET /admin/payments", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListPaymentRecords))))
	mux.Handle("GET /admin/articles", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListArticles))))
	mux.Handle("POST /admin/articles", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminCreateArticle))))
	mux.Handle("GET /admin/articles/{slug}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminGetArticle))))
	mux.Handle("PUT /admin/articles/{slug}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminUpdateArticle))))
	mux.Handle("DELETE /admin/articles/{slug}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminDeleteArticle))))
	mux.Handle("POST /admin/articles/{slug}/publish", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminPublishArticle))))
	mux.Handle("POST /admin/articles/{slug}/unpublish", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminUnpublishArticle))))
	// Public: Docs
	mux.HandleFunc("GET /public/docs/categories", r.handlePublicListDocCategoriesWithPages)
	mux.HandleFunc("GET /public/docs/pages/{slug}", r.handlePublicGetDocPage)
	// Admin: Doc Categories
	mux.Handle("GET /admin/docs/categories", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListDocCategories))))
	mux.Handle("POST /admin/docs/categories", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminCreateDocCategory))))
	mux.Handle("GET /admin/docs/categories/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminGetDocCategory))))
	mux.Handle("PUT /admin/docs/categories/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminUpdateDocCategory))))
	mux.Handle("DELETE /admin/docs/categories/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminDeleteDocCategory))))
	mux.Handle("POST /admin/docs/categories/{id}/publish", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminPublishDocCategory))))
	mux.Handle("POST /admin/docs/categories/{id}/unpublish", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminUnpublishDocCategory))))
	// Admin: Doc Pages
	mux.Handle("GET /admin/docs/pages", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListDocPages))))
	mux.Handle("POST /admin/docs/pages", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminCreateDocPage))))
	mux.Handle("GET /admin/docs/pages/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminGetDocPage))))
	mux.Handle("PUT /admin/docs/pages/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminUpdateDocPage))))
	mux.Handle("DELETE /admin/docs/pages/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminDeleteDocPage))))
	mux.Handle("POST /admin/docs/pages/{id}/publish", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminPublishDocPage))))
	mux.Handle("POST /admin/docs/pages/{id}/unpublish", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminUnpublishDocPage))))
	// Admin: Config Center - Software Configs
	mux.Handle("GET /admin/config-center/software", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListSoftwareConfigs))))
	mux.Handle("POST /admin/config-center/software", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminCreateSoftwareConfig))))
	mux.Handle("GET /admin/config-center/software/{code}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminGetSoftwareConfig))))
	mux.Handle("PUT /admin/config-center/software/{code}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminUpdateSoftwareConfig))))
	mux.Handle("DELETE /admin/config-center/software/{code}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminDeleteSoftwareConfig))))
	mux.Handle("POST /admin/config-center/software/{code}/tags", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminAddTag))))
	mux.Handle("DELETE /admin/config-center/software/{code}/tags/{tag}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminRemoveTag))))
	mux.Handle("GET /admin/config-center/software/{code}/templates", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListTemplates))))
	mux.Handle("POST /admin/config-center/software/{code}/templates", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminCreateTemplate))))
	mux.Handle("PUT /admin/config-center/templates/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminUpdateTemplate))))
	mux.Handle("DELETE /admin/config-center/templates/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminDeleteTemplate))))
	mux.Handle("GET /admin/config-center/global-vars", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListGlobalVars))))
	mux.Handle("POST /admin/config-center/global-vars", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminSetGlobalVar))))
	mux.Handle("DELETE /admin/config-center/global-vars/{key}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminDeleteGlobalVar))))

	// User: Config Center - Default config by tag
	mux.Handle("GET /api/v1/configs/default", authenticated(http.HandlerFunc(r.handleGetDefaultConfig)))
	mux.Handle("POST /api/v1/configs/sync", authenticated(http.HandlerFunc(r.handleSyncConfigs)))
	mux.Handle("GET /api/v1/configs/sync", authenticated(http.HandlerFunc(r.handlePullConfigs)))
	mux.Handle("POST /api/v1/configs/compare", authenticated(http.HandlerFunc(r.handleCompareConfigs)))
	mux.Handle("DELETE /api/v1/configs/sync/{uuid}", authenticated(http.HandlerFunc(r.handleDeleteSyncedConfig)))
	mux.Handle("GET /api/v1/configs/software-list", authenticated(http.HandlerFunc(r.handleListSoftware)))
	mux.Handle("GET /api/v1/configs/sync/status", authenticated(http.HandlerFunc(r.handleGetSyncStatus)))

	// Download Center: admin CRUD
	mux.Handle("GET /admin/download-center", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListDownloads))))
	mux.Handle("POST /admin/download-center", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminCreateDownload))))
	mux.Handle("GET /admin/download-center/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminGetDownload))))
	mux.Handle("PUT /admin/download-center/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminUpdateDownload))))
	mux.Handle("DELETE /admin/download-center/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminDeleteDownload))))
	mux.Handle("GET /admin/services", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminListServiceDirections))))
	mux.Handle("POST /admin/services", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminCreateServiceDirection))))
	mux.Handle("GET /admin/services/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminGetServiceDirection))))
	mux.Handle("PUT /admin/services/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminUpdateServiceDirection))))
	mux.Handle("DELETE /admin/services/{id}", authenticated(auth.RequireAdmin(http.HandlerFunc(r.handleAdminDeleteServiceDirection))))

	// Download Center: public version check (no auth required)
	mux.HandleFunc("GET /public/downloads/check", r.handlePublicVersionCheck)
	mux.HandleFunc("GET /public/downloads", r.handlePublicListDownloads)
	mux.HandleFunc("GET /public/services", r.handlePublicListServiceDirections)

	mux.HandleFunc("POST /api/ai/request", r.handleAIRequest)

	// 扫码登录过期清理：进程级后台 goroutine
	r.scanLogin.StartCleanup(context.Background())
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
	if payload.Role != "user" && payload.Role != "admin" && payload.Role != "distributor" {
		writeError(w, http.StatusBadRequest, "role must be user, admin, or distributor")
		return
	}

	if payload.Role == "admin" || payload.Role == "distributor" {
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

	if userID, found, lookupErr := r.findLocalUserIDByEmail(req.Context(), payload.Email); lookupErr != nil {
		slog.Warn("failed to resolve local user after password reset for sub2api token sync", "email", payload.Email, "error", lookupErr)
	} else if found {
		r.trySyncSub2APIAuthTokensWithPassword(req.Context(), userID, payload.Email, payload.NewPassword, "password_reset")
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
		slog.Error("resolveLocalAuthMeProfile error", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to resolve local session")
		return
	} else if handled {
		slog.Debug("auth/me resolved locally", "email", profile.Email)
		writeJSON(w, http.StatusOK, profile)
		return
	}

	slog.Debug("auth/me falling through to upstream proxy")
	r.handleAuthPassthrough(w, req, "/api/v1/auth/me")
}

// handleAuthRefreshPassthrough routes /auth/refresh through the refresh arbiter
// (handleAuthRefreshArbiter) instead of a bare upstream passthrough. The arbiter
// dedupes concurrent/multi-device refreshes and serves the cached current token
// pair without re-calling sub2api, so sub2api never sees a refresh-token reuse
// (which would revoke the whole token family and kick every device off).
func (r *routes) handleAuthRefreshPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleAuthRefreshArbiter(w, req)
}

// refreshEarlyRotateSkew is how long before the stored access_token's expiry the
// arbiter rotates instead of serving cached, so a client isn't handed a token
// that expires before its next request.
const refreshEarlyRotateSkew = 2 * time.Minute

// userRefreshLock returns a per-user mutex that serialises refresh rotations.
// Lazy-initialised; the map never shrinks (one entry per user is negligible).
func (r *routes) userRefreshLock(userID int64) *sync.Mutex {
	r.refreshMu.Lock()
	defer r.refreshMu.Unlock()
	if r.refreshUserMu == nil {
		r.refreshUserMu = make(map[int64]*sync.Mutex)
	}
	mu, ok := r.refreshUserMu[userID]
	if !ok {
		mu = &sync.Mutex{}
		r.refreshUserMu[userID] = mu
	}
	return mu
}

// accessTokenExpiry decodes a JWT's `exp` claim WITHOUT verifying the signature.
// Used only as a cache-freshness hint (sub2api remains authoritative); returns
// nil for anything that isn't a JWT carrying a numeric exp.
func accessTokenExpiry(token string) *time.Time {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Fall back to padded standard base64 for encoders that include padding.
		padded, padErr := base64.URLEncoding.DecodeString(parts[1])
		if padErr != nil {
			return nil
		}
		payload = padded
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp <= 0 {
		return nil
	}
	t := time.Unix(claims.Exp, 0).UTC()
	return &t
}

// defaultAccessTTL is the conservative lifetime the arbiter assumes when an
// upstream access_token can't be decoded as a JWT (sub2api always issues JWTs,
// so this is purely defensive). It keeps the vault's access_expires_at populated
// so dedup still works; the only cost of an inaccurate default is rotating a
// little early/late, never a correctness issue.
const defaultAccessTTL = 50 * time.Minute

// accessExpiryOrDefault returns the JWT exp when decodable, else now + a
// conservative default so the vault always records an expiry for dedup.
func accessExpiryOrDefault(token string) *time.Time {
	if t := accessTokenExpiry(token); t != nil {
		return t
	}
	fallback := time.Now().UTC().Add(defaultAccessTTL)
	return &fallback
}

// callSub2APIRefresh forwards a refresh_token to sub2api /api/v1/auth/refresh and
// returns the upstream status + raw body. Used only when the arbiter has decided
// a real rotation is necessary.
func (r *routes) callSub2APIRefresh(ctx context.Context, refreshToken string) (int, []byte, error) {
	bodyBytes, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, nil, err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.ContentLength = int64(len(bodyBytes))

	resp, err := r.proxyClient.Do(ctx, upstreamReq, "/api/v1/auth/refresh")
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return resp.StatusCode, nil, readErr
	}
	return resp.StatusCode, respBody, nil
}

// handleAuthRefreshArbiter is the multi-device-safe refresh endpoint.
//
// The caller contract is unchanged: POST {refresh_token} → {access_token,
// refresh_token, expires_in}. Internally it serialises per user and only calls
// sub2api when the cached access_token is near expiry, serving the cached pair
// otherwise — so sub2api never observes a refresh-token reuse (which would
// revoke the whole token family and kick every device of that user offline).
//
// A refresh_token equal to the current OR the immediately-previous stored value
// is accepted (one-generation grace window). Anything older is rejected so the
// client re-authenticates rather than risk a replay-detection nuke upstream.
func (r *routes) handleAuthRefreshArbiter(w http.ResponseWriter, req *http.Request) {
	if r.sub2api == nil || !r.sub2api.IsConfigured() || r.proxyClient == nil {
		// No upstream configured — preserve the original passthrough behaviour.
		r.handleAuthPassthrough(w, req, "/api/v1/auth/refresh")
		return
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to read refresh request")
		return
	}
	_ = req.Body.Close()

	presentedRefresh := extractAuthRefreshTokenFromRequestBody(bodyBytes)
	if strings.TrimSpace(presentedRefresh) == "" {
		writeError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	userID, found, err := r.sub2api.FindUserIDByRefreshOrPrev(req.Context(), presentedRefresh)
	if err != nil {
		slog.Error("refresh arbiter: resolve user failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to resolve session")
		return
	}
	if !found {
		// Unknown or too-stale token. Do NOT forward upstream — we can't identify
		// the family, and forwarding a stale token risks a replay-detection nuke.
		writeError(w, http.StatusUnauthorized, "refresh token is no longer valid")
		return
	}

	lock := r.userRefreshLock(userID)
	lock.Lock()
	defer lock.Unlock()

	vault, err := r.sub2api.LoadVault(req.Context(), userID)
	if err != nil {
		slog.Error("refresh arbiter: load vault failed", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to resolve session")
		return
	}
	if vault == nil || !vault.HasRefresh {
		// Vault vanished between resolution and the lock (concurrent clear) or has
		// no usable refresh — force re-authentication.
		writeError(w, http.StatusUnauthorized, "refresh token is no longer valid")
		return
	}

	now := time.Now().UTC()
	needsRotate := !vault.HasAccessExpires || now.After(vault.AccessExpiresAt.Add(-refreshEarlyRotateSkew))

	if !needsRotate {
		// Cached access_token still good: serve the CURRENT pair WITHOUT calling
		// sub2api. This is the dedup path that prevents replay-detection nukes —
		// regardless of whether the caller presented the current or previous
		// refresh_token, they converge onto the current pair.
		expiresIn := int(time.Until(vault.AccessExpiresAt).Seconds())
		if expiresIn < 1 {
			expiresIn = 1
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "success",
			"data": map[string]any{
				"access_token":  vault.AccessToken,
				"refresh_token": vault.RefreshToken,
				"expires_in":    expiresIn,
				"token_type":    "Bearer",
			},
		})
		return
	}

	// Rotation needed — always rotate using the CURRENT refresh, never the
	// possibly-previous token the caller presented. Single-flight under the lock
	// guarantees only one sub2api call per rotation cycle.
	status, respBody, callErr := r.callSub2APIRefresh(req.Context(), vault.RefreshToken)
	if callErr != nil {
		slog.Error("refresh arbiter: upstream refresh call failed", "user_id", userID, "error", callErr)
		writeError(w, http.StatusBadGateway, "failed to refresh session")
		return
	}

	if status < 200 || status >= 300 {
		// sub2api rejected the rotation — the family is dead/invalid. Clear the
		// vault so the user re-authenticates instead of us repeatedly forwarding
		// a doomed token (which would keep tripping replay detection).
		if clearErr := r.sub2api.ClearVault(req.Context(), userID); clearErr != nil {
			slog.Warn("refresh arbiter: clear vault after failed rotation failed", "user_id", userID, "error", clearErr)
		}
		slog.Warn("refresh arbiter: upstream rejected rotation; vault cleared", "user_id", userID, "status", status)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(respBody)
		return
	}

	// Success: capture the rotated pair (UpsertToken maintains prev_refresh_token
	// automatically) and extend the local session.
	accessToken, refreshPtr, _, upstreamUserID, ok := extractSub2APITokensFromAuthResponse(respBody)
	if !ok {
		slog.Error("refresh arbiter: could not parse upstream refresh response", "user_id", userID)
		writeError(w, http.StatusBadGateway, "failed to parse refresh response")
		return
	}
	if err := r.sub2api.CaptureTokens(req.Context(), sub2apiauth.UpsertTokenInput{
		UserID:          userID,
		UpstreamUserID:  upstreamUserID,
		AccessToken:     accessToken,
		RefreshToken:    refreshPtr,
		AccessExpiresAt: accessExpiryOrDefault(accessToken),
	}); err != nil {
		slog.Error("refresh arbiter: capture rotated tokens failed", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to persist refreshed session")
		return
	}
	if extendErr := r.extendLocalSessionExpiry(req.Context(), userID); extendErr != nil {
		slog.Warn("refresh arbiter: extend local session failed", "user_id", userID, "error", extendErr)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBody)
}

// EnsureFreshUpstreamAccessToken makes sure the user's cached sub2api access_token
// is still usable, rotating it via sub2api when it has expired (or is within
// refreshEarlyRotateSkew of expiring). It is the reusable core of the refresh
// arbiter, exposed through UserResolver so the passthrough layer
// (Gateway.ReplaceAuthHeader) can keep data endpoints (user/profile, api-keys,
// groups, usage, ...) working when a cached credential has simply aged out —
// instead of surfacing sub2api's 401 INVALID_TOKEN to the client.
//
// Lenient by design: no stored token / no refresh_token → nil (the caller then
// falls back to GetBearerTokenByUserID and its ErrTokenNotFound handling). Only a
// failed upstream rotation clears the vault (forcing re-authentication) and
// returns an error.
func (r *routes) EnsureFreshUpstreamAccessToken(ctx context.Context, userID int64) error {
	if r.sub2api == nil || !r.sub2api.IsConfigured() || r.proxyClient == nil || userID <= 0 {
		return nil
	}

	lock := r.userRefreshLock(userID)
	lock.Lock()
	defer lock.Unlock()

	vault, err := r.sub2api.LoadVault(ctx, userID)
	if err != nil {
		return fmt.Errorf("load upstream token vault: %w", err)
	}
	if vault == nil || !vault.HasRefresh {
		return nil
	}

	now := time.Now().UTC()
	// Only rotate proactively when the expiry is KNOWN and past/near. A NULL
	// access_expires_at (legacy/migrated rows, or capture paths that couldn't
	// decode an expiry) is treated as "trust the stored token" — this path runs
	// on every passthrough request, so rotating on unknown expiry would hammer
	// sub2api. Such tokens self-heal via the explicit /auth/refresh flow.
	needsRotate := vault.HasAccessExpires && now.After(vault.AccessExpiresAt.Add(-refreshEarlyRotateSkew))
	if !needsRotate {
		return nil
	}

	status, respBody, callErr := r.callSub2APIRefresh(ctx, vault.RefreshToken)
	if callErr != nil {
		return fmt.Errorf("upstream refresh call: %w", callErr)
	}
	if status < 200 || status >= 300 {
		// sub2api rejected rotation — the family is dead. Clear the vault so the
		// user re-authenticates instead of us repeatedly forwarding a doomed token.
		if clearErr := r.sub2api.ClearVault(ctx, userID); clearErr != nil {
			slog.Warn("ensure fresh upstream token: clear vault after failed rotation failed", "user_id", userID, "error", clearErr)
		}
		slog.Warn("ensure fresh upstream token: upstream rejected rotation; vault cleared", "user_id", userID, "status", status)
		return fmt.Errorf("upstream rejected token rotation (status %d)", status)
	}

	accessToken, refreshPtr, _, upstreamUserID, ok := extractSub2APITokensFromAuthResponse(respBody)
	if !ok {
		return fmt.Errorf("parse upstream refresh response")
	}
	if err := r.sub2api.CaptureTokens(ctx, sub2apiauth.UpsertTokenInput{
		UserID:          userID,
		UpstreamUserID:  upstreamUserID,
		AccessToken:     accessToken,
		RefreshToken:    refreshPtr,
		AccessExpiresAt: accessExpiryOrDefault(accessToken),
	}); err != nil {
		return fmt.Errorf("persist rotated tokens: %w", err)
	}
	slog.Info("ensure fresh upstream token: rotated expired credential", "user_id", userID)
	return nil
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

func (r *routes) handleOpsDashboardSnapshotPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/admin/ops/dashboard/snapshot-v2")
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

	profile, err := r.loadLocalUserProfile(ctx, userID)
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
	r.handleDashboardPassthrough(w, req, "/api/v1/api-keys")
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
		r.handleDashboardPassthrough(w, req, "/api/v1/api-keys")
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

	filteredPayload, statusCode, headers, handled, err := r.filteredProxyJSONResponse(w, req, "/api/v1/api-keys", func(payload any) (any, error) {
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

// ----- alianggate app client compatibility handlers -----

func (r *routes) handleUserProfilePassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/user/profile")
}

func (r *routes) handleUserProfileUpdatePassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/user")
}

func (r *routes) handleUserSubSummaryPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/subscriptions/summary")
}

func (r *routes) handleUserSubProgressPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/subscriptions/progress")
}

func (r *routes) handleUserGroupsAvailablePassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleFilteredGroupsAvailablePassthrough(w, req)
}

func (r *routes) handleUserAPIKeysPassthrough(w http.ResponseWriter, req *http.Request) {
	// 不过滤：用户能看到自己在 sub2api 的所有 keys（换套餐后旧 group 的 keys 仍可见，不被 active-subscription 过滤）。
	r.handleDashboardPassthrough(w, req, "/api/v1/api-keys")
}

func (r *routes) handleUserCodeRedeemPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/redeem")
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
		upstreamPath := "/api/v1/api-keys/" + req.PathValue("id")
		if req.Method == http.MethodDelete && r.rejectProtectedAPIKeyDelete(w, req, upstreamPath) {
			return
		}
		r.handleDashboardPassthrough(w, req, upstreamPath)
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

	upstreamPath := "/api/v1/api-keys/" + req.PathValue("id")
	if req.Method == http.MethodDelete {
		payload, err := r.loadUpstreamJSONPayload(req, upstreamPath)
		if err != nil {
			if errors.Is(err, sub2apiauth.ErrTokenNotFound) {
				writeError(w, http.StatusUnauthorized, "upstream session unavailable")
				return
			}
			writeError(w, http.StatusBadGateway, "failed to fetch api key")
			return
		}
		allowed, err := isAPIKeyPayloadAuthorized(payload, authorizedGroupIDs)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch api key")
			return
		}
		if !allowed {
			writeError(w, http.StatusForbidden, "group access forbidden")
			return
		}
		protected, err := isProtectedAPIKeyPayload(payload)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch api key")
			return
		}
		if protected {
			writeError(w, http.StatusForbidden, "auto-key cannot be deleted")
			return
		}
		r.handleDashboardPassthrough(w, req, upstreamPath)
		return
	}

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

func (r *routes) rejectProtectedAPIKeyDelete(w http.ResponseWriter, req *http.Request, upstreamPath string) bool {
	payload, err := r.loadUpstreamJSONPayload(req, upstreamPath)
	if err != nil {
		if errors.Is(err, sub2apiauth.ErrTokenNotFound) {
			writeError(w, http.StatusUnauthorized, "upstream session unavailable")
			return true
		}
		writeError(w, http.StatusBadGateway, "failed to fetch api key")
		return true
	}
	protected, err := isProtectedAPIKeyPayload(payload)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch api key")
		return true
	}
	if protected {
		writeError(w, http.StatusForbidden, "auto-key cannot be deleted")
		return true
	}
	return false
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
			BillingType:      packageGroupBillingType(group),
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
	if authUser, ok := auth.UserFromContext(req.Context()); ok {
		switch authUser.Role {
		case "distributor":
			packages = filterAdminPackagesByLevel(packages, packageLevelDistributor)
		}
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
	if authUser, ok := auth.UserFromContext(req.Context()); ok {
		if authUser.Role == "distributor" && pkg.Level != packageLevelDistributor {
			writeError(w, http.StatusForbidden, "this role can only access distributor-level packages")
			return
		}
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
	isVisible, isPublished := packageFlagsForCreate(normalized)

	tx, err := r.db.BeginTx(req.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create package")
		return
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	tierID, err := db.InsertID(req.Context(), r.sqlDialect, tx, `
		INSERT INTO als_tiers(code, name, level, price_micros, value_type, value_amount, rate, min_topup_micros, max_topup_micros, description, features_json, is_enabled, is_visible, is_published, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "id", normalized.Code, normalized.Name, normalized.Level, normalized.PriceMicros, normalized.ValueType, normalized.ValueAmount, normalized.Rate, normalized.MinTopupMicros, normalized.MaxTopupMicros, normalized.Description, normalized.FeaturesJSON, isPublished, isVisible, isPublished, now, now)
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

	currentPkg, err := r.loadAdminPackageByCode(req.Context(), packageCode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load package")
		return
	}
	if normalized.Level == "" {
		normalized.Level = currentPkg.Level
	}
	isVisibleVal, isPublishedVal := packageFlagsForUpdate(normalized, currentPkg)

	tx, err := r.db.BeginTx(req.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update package")
		return
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `UPDATE als_tiers SET name = ?, level = ?, price_micros = ?, value_type = ?, value_amount = ?, rate = ?, min_topup_micros = ?, max_topup_micros = ?, description = ?, features_json = ?, is_enabled = ?, is_visible = ?, is_published = ?, updated_at = ? WHERE id = ?;`), normalized.Name, normalized.Level, normalized.PriceMicros, normalized.ValueType, normalized.ValueAmount, normalized.Rate, normalized.MinTopupMicros, normalized.MaxTopupMicros, normalized.Description, normalized.FeaturesJSON, isPublishedVal, isVisibleVal, isPublishedVal, now, tierID)
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

func (r *routes) handleAdminDeletePackage(w http.ResponseWriter, req *http.Request) {
	packageCode := strings.TrimSpace(req.PathValue("code"))
	if packageCode == "" {
		writeError(w, http.StatusBadRequest, "package code is required")
		return
	}

	result, err := r.db.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `DELETE FROM als_tiers WHERE code = ?;`), packageCode)
	if err != nil {
		writeError(w, http.StatusConflict, "package cannot be deleted because it is referenced by existing records")
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete package")
		return
	}
	if affected == 0 {
		writeError(w, http.StatusNotFound, "package not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "code": packageCode})
}

func (r *routes) handleSubscriptionsSummaryPassthrough(w http.ResponseWriter, req *http.Request) {
	localSessionToken, err := extractBearerToken(req.Header.Get("Authorization"))
	if err == nil {
		userID, found, err := r.findLocalUserIDBySessionToken(req.Context(), localSessionToken)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve local session")
			return
		}
		if found {
			payload, err := r.loadLocalPackageSubscriptionsSummary(req.Context(), userID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to load package subscriptions")
				return
			}
			writeJSON(w, http.StatusOK, payload)
			return
		}
	}

	r.handleDashboardPassthrough(w, req, "/api/v1/subscriptions/summary")
}

func (r *routes) handleSubscriptionsActivePassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/subscriptions/active")
}

func (r *routes) handleSubscriptionsAllPassthrough(w http.ResponseWriter, req *http.Request) {
	r.handleDashboardPassthrough(w, req, "/api/v1/subscriptions")
}

func (r *routes) loadLocalPackageSubscriptionsSummary(ctx context.Context, userID int64) (map[string]any, error) {
	rows, err := r.db.QueryContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT
			s.id,
			s.status,
			s.started_at,
			s.expires_at,
			t.code,
			t.name,
			t.price_micros,
			t.value_type,
			t.value_amount,
			t.description,
			t.features_json,
			tgb.group_id
		FROM als_subscriptions s
		JOIN als_tiers t ON t.id = s.tier_id
		LEFT JOIN als_tier_group_bindings tgb ON tgb.tier_id = t.id
		WHERE s.user_id = ?
			AND s.status = 'active'
			AND s.ended_at IS NULL
		ORDER BY s.started_at DESC, s.id DESC, tgb.group_id ASC;
	`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type packageSubscription struct {
		ID          int64
		Status      string
		StartedAt   string
		ExpiresAt   string
		PackageCode string
		PackageName string
		PriceMicros int64
		ValueType   string
		ValueAmount int64
		Description string
		Features    []string
		GroupIDs    []int64
	}

	subscriptions := make([]packageSubscription, 0)
	indexBySubscriptionID := make(map[int64]int)
	for rows.Next() {
		var (
			subscriptionID int64
			status         string
			startedAt      string
			expiresAt      sql.NullString
			packageCode    string
			packageName    string
			priceMicros    int64
			valueType      string
			valueAmount    int64
			description    string
			featuresJSON   string
			groupID        sql.NullInt64
		)
		if err := rows.Scan(&subscriptionID, &status, &startedAt, &expiresAt, &packageCode, &packageName, &priceMicros, &valueType, &valueAmount, &description, &featuresJSON, &groupID); err != nil {
			return nil, err
		}

		expiresAtValue := ""
		if expiresAt.Valid {
			expiresAtValue = expiresAt.String
		}

		idx, found := indexBySubscriptionID[subscriptionID]
		if !found {
			idx = len(subscriptions)
			indexBySubscriptionID[subscriptionID] = idx
			subscriptions = append(subscriptions, packageSubscription{
				ID:          subscriptionID,
				Status:      status,
				StartedAt:   startedAt,
				ExpiresAt:   expiresAtValue,
				PackageCode: packageCode,
				PackageName: packageName,
				PriceMicros: priceMicros,
				ValueType:   valueType,
				ValueAmount: valueAmount,
				Description: description,
				Features:    parseFeaturesJSON(featuresJSON),
				GroupIDs:    []int64{},
			})
		}
		if groupID.Valid {
			subscriptions[idx].GroupIDs = append(subscriptions[idx].GroupIDs, groupID.Int64)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	items := make([]any, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		items = append(items, map[string]any{
			"id":           subscription.ID,
			"group_id":     subscription.PackageCode,
			"group_name":   subscription.PackageName,
			"tier_code":    subscription.PackageCode,
			"tier_name":    subscription.PackageName,
			"package_code": subscription.PackageCode,
			"package_name": subscription.PackageName,
			"group_ids":    subscription.GroupIDs,
			"status":       subscription.Status,
			"started_at":   subscription.StartedAt,
			"expires_at":   packageSubscriptionExpiresAt(subscription.StartedAt, subscription.ExpiresAt, subscription.ValueType, subscription.ValueAmount),
			"price_micros": subscription.PriceMicros,
			"value_type":   subscription.ValueType,
			"value_amount": subscription.ValueAmount,
			"description":  subscription.Description,
			"features":     subscription.Features,
			"source":       "package",
		})
	}

	return map[string]any{
		"data": map[string]any{
			"active_count":   len(items),
			"total_used_usd": 0,
			"subscriptions":  items,
		},
	}, nil
}

func packageSubscriptionExpiresAt(startedAt string, expiresAt string, valueType string, valueAmount int64) string {
	if parsed, ok := parseSubscriptionTime(expiresAt); ok {
		return parsed.UTC().Format(time.RFC3339)
	}
	if strings.TrimSpace(valueType) != "days" || valueAmount <= 0 {
		return ""
	}

	if parsed, ok := parseSubscriptionTime(startedAt); ok {
		return parsed.UTC().AddDate(0, 0, int(valueAmount)).Format(time.RFC3339)
	}
	return ""
}

func parseSubscriptionTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z"} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
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

	slog.Debug("auth passthrough", "method", req.Method, "upstream", upstreamPath)

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
		slog.Debug("auth login request", "email", requestEmail)
	}
	if upstreamPath == "/api/v1/auth/refresh" && len(requestBody) > 0 {
		requestRefreshToken = extractAuthRefreshTokenFromRequestBody(requestBody)
	}

	forwarded := req.Clone(req.Context())
	forwarded.Header = cloneHeaders(req.Header)
	forwarded.Header.Del("X-User-Id")
	if err := r.sub2api.ReplaceAuthHeader(req.Context(), forwarded.Header, r); err != nil {
		if errors.Is(err, sub2apiauth.ErrTokenNotFound) {
			slog.Warn("auth passthrough: upstream session unavailable", "path", upstreamPath)
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

	slog.Debug("auth passthrough upstream response", "path", upstreamPath, "status", resp.StatusCode)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 && (upstreamPath == "/api/v1/auth/login" || upstreamPath == "/api/v1/auth/refresh" || upstreamPath == "/api/v1/auth/register") {
		localUserID, found, err := r.captureSub2APITokens(req.Context(), req, requestEmail, requestRefreshToken, responseBody)
		if err != nil {
			slog.Error("captureSub2APITokens failed", "path", upstreamPath, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to persist auth session")
			return
		}
		slog.Debug("captureSub2APITokens result", "path", upstreamPath, "found", found, "user_id", localUserID)
		if upstreamPath == "/api/v1/auth/login" && found {
			sessionToken, sessionErr := r.createLocalSessionToken(req.Context(), localUserID)
			if sessionErr != nil {
				writeError(w, http.StatusInternalServerError, "failed to create local session")
				return
			}
			slog.Info("local session created", "user_id", localUserID, "email", requestEmail)
			localProfile, profileErr := r.loadLocalUserProfile(req.Context(), localUserID)
			if profileErr != nil {
				slog.Error("failed to load local profile after login", "user_id", localUserID, "error", profileErr)
				writeError(w, http.StatusInternalServerError, "failed to load local profile")
				return
			}
			responseBody, err = injectLocalSessionIntoAuthResponse(responseBody, sessionToken, localProfile)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to finalize login response")
				return
			}
			resp.Body = io.NopCloser(bytes.NewReader(responseBody))
			resp.ContentLength = int64(len(responseBody))
			resp.Header.Set("Content-Length", strconv.Itoa(len(responseBody)))
		}
		if upstreamPath == "/api/v1/auth/refresh" && found {
			if extendErr := r.extendLocalSessionExpiry(req.Context(), localUserID); extendErr != nil {
				slog.Warn("failed to extend session on refresh", "error", extendErr)
			} else {
				slog.Debug("session extended on refresh", "user_id", localUserID)
			}
		}
	}

	if err := proxy.CopyResponse(w, resp); err != nil {
		slog.Error("proxy auth response copy failed", "path", upstreamPath, "error", err)
		return
	}
}

func (r *routes) captureSub2APITokens(ctx context.Context, req *http.Request, requestEmail, requestRefreshToken string, responseBody []byte) (int64, bool, error) {
	accessToken, refreshToken, _, upstreamUserID, ok := extractSub2APITokensFromAuthResponse(responseBody)
	if !ok {
		return 0, false, nil
	}

	authIdentity := extractAuthIdentityFromResponse(responseBody)
	if upstreamUserID != nil && *upstreamUserID > 0 {
		authIdentity.ID = *upstreamUserID
	}
	localUserID, found, err := r.resolveLocalUserIDForAuthTokens(ctx, upstreamUserID, authIdentity.Email, requestEmail, req.Header.Get("Authorization"), requestRefreshToken, refreshToken)
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

	if err := r.sub2api.CaptureTokens(ctx, sub2apiauth.UpsertTokenInput{
		UserID:          localUserID,
		UpstreamUserID:  upstreamUserID,
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		AccessExpiresAt: accessExpiryOrDefault(accessToken),
	}); err != nil {
		return 0, false, err
	}

	return localUserID, true, nil
}

func (r *routes) resolveLocalUserIDForAuthTokens(ctx context.Context, upstreamUserID *int64, responseEmail, requestEmail, authHeader, requestRefreshToken string, refreshToken *string) (int64, bool, error) {
	if upstreamUserID != nil && *upstreamUserID > 0 {
		if userID, found, err := r.findLocalUserIDByUpstreamUserID(ctx, *upstreamUserID); err != nil {
			return 0, false, err
		} else if found {
			return userID, true, nil
		}
		if _, err := r.loadLocalUserProfile(ctx, *upstreamUserID); err == nil {
			return *upstreamUserID, true, nil
		} else if !errors.Is(err, user.ErrUserNotFound) {
			return 0, false, err
		}
	}

	for _, email := range []string{responseEmail, requestEmail} {
		if strings.TrimSpace(email) == "" {
			continue
		}
		userID, found, err := r.findLocalUserIDByEmail(ctx, email)
		if err != nil {
			return 0, false, err
		}
		if found {
			if upstreamUserID != nil && *upstreamUserID > 0 && userID != *upstreamUserID {
				return 0, false, fmt.Errorf("local user id %d does not match sub2api user id %d for %s", userID, *upstreamUserID, strings.TrimSpace(email))
			}
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

func (r *routes) loadLocalUserProfile(ctx context.Context, userID int64) (*user.UserProfile, error) {
	var profile user.UserProfile
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT id, email, name, role, created_at, updated_at
		FROM als_users
		WHERE id = ?
		LIMIT 1;
	`), userID).Scan(&profile.ID, &profile.Email, &profile.Name, &profile.Role, &profile.CreatedAt, &profile.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, user.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query local user profile: %w", err)
	}
	return &profile, nil
}

func (r *routes) loadLocalUserForRoleUpdate(ctx context.Context, userID int64, email string) (localUserRoleTarget, error) {
	var target localUserRoleTarget
	if userID > 0 {
		err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
			SELECT id, email, name, role
			FROM als_users
			WHERE id = ?
			LIMIT 1;
		`), userID).Scan(&target.ID, &target.Email, &target.Name, &target.Role)
		if err != nil {
			return localUserRoleTarget{}, err
		}
		return target, nil
	}

	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT id, email, name, role
		FROM als_users
		WHERE LOWER(email) = LOWER(?)
		LIMIT 1;
	`), strings.TrimSpace(email)).Scan(&target.ID, &target.Email, &target.Name, &target.Role)
	if err != nil {
		return localUserRoleTarget{}, err
	}
	return target, nil
}

func (r *routes) hasOtherLocalAdmin(ctx context.Context, userID int64) (bool, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT COUNT(*)
		FROM als_users
		WHERE role = 'admin'
			AND id != ?;
	`), userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func isSupportedLocalRole(role string) bool {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case "user", "admin", "distributor":
		return true
	default:
		return false
	}
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

// loadAuthenticatedUserByID loads the full AuthenticatedUser (id/email/name/
// role) for a local als_users row. Used by the scan-login auth paths that
// resolve a userID via a non-st_ credential (the phone's stored sub2api
// access_token) and need to inject the user into the request context.
func (r *routes) loadAuthenticatedUserByID(ctx context.Context, userID int64) (*auth.AuthenticatedUser, error) {
	var u auth.AuthenticatedUser
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT id, email, name, role FROM als_users WHERE id = ?;
	`), userID).Scan(&u.ID, &u.Email, &u.Name, &u.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load authenticated user by id: %w", err)
	}
	return &u, nil
}

// resolveScanUserID resolves the local als_users.id behind a scan-login App
// bearer token. Accepts EITHER an official-website st_ session (the website's
// own frontend) OR the sub2api access_token the phone app holds (looked up in
// the locally stored als_sub2api_auth_tokens row, which the refresh arbiter
// keeps current). Returns ok=false when the token matches neither.
func (r *routes) resolveScanUserID(ctx context.Context, token string) (int64, bool) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return 0, false
	}
	if id, ok, err := r.findLocalUserIDBySessionToken(ctx, trimmed); err == nil && ok {
		return id, true
	}
	if id, ok, err := r.findLocalUserIDByStoredAccessToken(ctx, trimmed); err == nil && ok {
		return id, true
	}
	return 0, false
}

// requireUserForScan authenticates the scan-login App (phone or website). It
// resolves the user via resolveScanUserID (st_ OR stored access_token) and
// injects the AuthenticatedUser into the context, so the existing scan handlers
// (which call auth.UserFromContext) work unchanged.
func (r *routes) requireUserForScan(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		token := strings.TrimSpace(strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer "))
		userID, ok := r.resolveScanUserID(req.Context(), token)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid or expired session")
			return
		}
		user, err := r.loadAuthenticatedUserByID(req.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to authenticate")
			return
		}
		if user == nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired session")
			return
		}
		next.ServeHTTP(w, req.WithContext(auth.WithUser(req.Context(), *user)))
	})
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

func (r *routes) findLocalUserIDByUpstreamUserID(ctx context.Context, upstreamUserID int64) (int64, bool, error) {
	if upstreamUserID <= 0 {
		return 0, false, nil
	}

	var userID int64
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT user_id
		FROM als_sub2api_auth_tokens
		WHERE upstream_user_id = ?
		LIMIT 1;
	`), upstreamUserID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("query local user by upstream user id: %w", err)
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

// FindUserIDBySession implements sub2api.UserResolver.
func (r *routes) FindUserIDBySession(ctx context.Context, sessionToken string) (int64, bool, error) {
	return r.findLocalUserIDBySessionToken(ctx, sessionToken)
}

// FindUserRoleByID implements sub2api.UserResolver.
func (r *routes) FindUserRoleByID(ctx context.Context, id int64) (string, bool, error) {
	return r.findLocalUserRoleByID(ctx, id)
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
	ID    int64
	Email string
	Name  string
	Role  string
}

type localUserRoleTarget struct {
	ID             int64
	UpstreamUserID *int64
	Email          string
	Name           string
	Role           string
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
			if id, ok := int64FromAny(userObj["id"]); ok {
				identity.ID = id
			}
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
		if identity.ID <= 0 {
			if id, ok := int64FromAny(candidate["id"]); ok {
				identity.ID = id
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

	slog.Debug("dashboard passthrough", "method", req.Method, "upstream", upstreamPath)

	forwarded := req.Clone(req.Context())
	forwarded.Header = cloneHeaders(req.Header)
	forwarded.Header.Del("X-User-Id")
	if err := r.sub2api.ReplaceAuthHeader(req.Context(), forwarded.Header, r); err != nil {
		if errors.Is(err, sub2apiauth.ErrTokenNotFound) {
			slog.Warn("dashboard passthrough: upstream session unavailable", "path", upstreamPath)
			writeError(w, http.StatusUnauthorized, "upstream session unavailable")
			return
		}
		slog.Error("dashboard passthrough: failed to resolve upstream session", "path", upstreamPath, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to resolve upstream session")
		return
	}

	resp, err := r.proxyClient.Do(req.Context(), forwarded, upstreamPath)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to proxy dashboard request")
		return
	}

	slog.Debug("dashboard passthrough upstream response", "path", upstreamPath, "status", resp.StatusCode)

	if err := proxy.CopyResponse(w, resp); err != nil {
		slog.Error("proxy dashboard response copy failed", "path", upstreamPath, "error", err)
		return
	}
}

// handleUpstreamPassthrough returns a handler that proxies the request to the specified upstream path.
func (r *routes) handleUpstreamPassthrough(upstreamPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r.handleDashboardPassthrough(w, req, upstreamPath)
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
		if identity.ID > 0 && userID != identity.ID {
			return 0, false, fmt.Errorf("local user id %d does not match sub2api user id %d for %s", userID, identity.ID, email)
		}
		return userID, true, nil
	}

	name := strings.TrimSpace(identity.Name)
	if name == "" {
		name = strings.TrimSpace(strings.Split(email, "@")[0])
		if name == "" {
			name = email
		}
	}
	role := "user"

	if identity.ID > 0 {
		if _, err := r.db.ExecContext(ctx, db.Rebind(r.sqlDialect, `
			INSERT INTO als_users(id, email, name, role)
			VALUES (?, ?, ?, ?);
		`), identity.ID, email, name, role); err != nil {
			return 0, false, fmt.Errorf("create local auth user: %w", err)
		}
		return identity.ID, true, nil
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

func (r *routes) extendLocalSessionExpiry(ctx context.Context, userID int64) error {
	newExpiry := time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02 15:04:05")
	_, err := r.db.ExecContext(ctx, db.Rebind(r.sqlDialect, `
		UPDATE als_sessions
		SET expires_at = ?
		WHERE user_id = ?
		  AND revoked_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1;
	`), newExpiry, userID)
	if err != nil {
		return fmt.Errorf("extend session expiry: %w", err)
	}
	return nil
}

func injectLocalSessionIntoAuthResponse(body []byte, sessionToken string, profile *user.UserProfile) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	payload["session_token"] = sessionToken
	payload["access_token"] = sessionToken
	overlayLocalProfile(payload, profile)
	if data, ok := payload["data"].(map[string]any); ok {
		data["session_token"] = sessionToken
		data["access_token"] = sessionToken
		overlayLocalProfile(data, profile)
		payload["data"] = data
	}
	return json.Marshal(payload)
}

func overlayLocalProfile(target map[string]any, profile *user.UserProfile) {
	if target == nil || profile == nil {
		return
	}

	target["id"] = profile.ID
	target["email"] = profile.Email
	target["name"] = profile.Name
	target["role"] = profile.Role

	userObj, _ := target["user"].(map[string]any)
	if userObj == nil {
		userObj = make(map[string]any)
	}
	userObj["id"] = profile.ID
	userObj["email"] = profile.Email
	userObj["name"] = profile.Name
	userObj["role"] = profile.Role
	target["user"] = userObj
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

	r.trySyncSub2APIAuthTokensWithPassword(req.Context(), authUser.ID, authUser.Email, payload.NewPassword, "password_change")

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

	r.trySyncSub2APIAuthTokensWithPassword(req.Context(), authUser.ID, authUser.Email, payload.NewPassword, "password_set")

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
		if errors.Is(err, apikey.ErrProtectedAPIKey) {
			writeError(w, http.StatusForbidden, "auto-key cannot be deleted")
			return
		}
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
		SELECT code, name, price_micros, value_type, value_amount, description, features_json, is_published, rate, min_topup_micros, max_topup_micros
		FROM als_tiers
		WHERE is_visible = ?
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
			isPublished  bool
			rate         sql.NullFloat64
			minTopup     sql.NullInt64
			maxTopup     sql.NullInt64
		)
		if err := rows.Scan(&code, &name, &priceMicros, &valueType, &valueAmount, &description, &featuresJSON, &isPublished, &rate, &minTopup, &maxTopup); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list packages")
			return
		}
		packages = append(packages, publicPackageResponse{
			Code:           code,
			Name:           name,
			PriceMicros:    priceMicros,
			ValueType:      valueType,
			ValueAmount:    valueAmount,
			Rate:           rate.Float64,
			MinTopupMicros: minTopup.Int64,
			MaxTopupMicros: maxTopup.Int64,
			Description:    description,
			Features:       parseFeaturesJSON(featuresJSON),
			IsPublished:    isPublished,
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
		slog.Error("handlePublicListArticles: list published articles", "error", err)
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
		slog.Error("handlePublicGetArticle: get article", "slug", slug, "error", err)
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
		slog.Error("handleAdminListArticles: list articles", "error", err)
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
		slog.Error("handleAdminCreateArticle: create article", "slug", payload.Slug, "error", err)
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
		slog.Error("handleAdminGetArticle: find article", "slug", slug, "error", err)
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
		slog.Error("handleAdminUpdateArticle: find article", "slug", slug, "error", err)
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
		slog.Error("handleAdminUpdateArticle: update article", "slug", slug, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update article")
		return
	}

	refreshed, err := r.articleSvc.GetArticleByID(req.Context(), updatedArticle.ID)
	if errors.Is(err, article.ErrArticleNotFound) {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}
	if err != nil {
		slog.Error("handleAdminUpdateArticle: get article after update", "id", updatedArticle.ID, "error", err)
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
		slog.Error("handleAdminDeleteArticle: delete article", "slug", slug, "error", err)
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
		slog.Error("handleAdminPublishArticle: publish article", "slug", slug, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to publish article")
		return
	}

	updated, err := r.findAdminArticleBySlug(req.Context(), slug)
	if errors.Is(err, article.ErrArticleNotFound) {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}
	if err != nil {
		slog.Error("handleAdminPublishArticle: find article after publish", "slug", slug, "error", err)
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
		slog.Error("handleAdminUnpublishArticle: unpublish article", "slug", slug, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to unpublish article")
		return
	}

	updated, err := r.findAdminArticleBySlug(req.Context(), slug)
	if errors.Is(err, article.ErrArticleNotFound) {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}
	if err != nil {
		slog.Error("handleAdminUnpublishArticle: find article after unpublish", "slug", slug, "error", err)
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
	if !pkg.IsPublished {
		writeError(w, http.StatusBadRequest, "package is not published")
		return
	}
	if err := r.validatePackageGroupBindings(req.Context(), adminPackageRequest{
		GroupIDs:    pkg.GroupIDs,
		ValueType:   pkg.ValueType,
		ValueAmount: pkg.ValueAmount,
	}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// 加量包（balance + rate>0）按用户填的金额收款；其他套餐用 tier 固定价。
	priceMicros := pkg.PriceMicros
	if pkg.ValueType == "balance" && pkg.Rate > 0 {
		if payload.AmountMicros <= 0 {
			writeError(w, http.StatusBadRequest, "amount_micros is required for top-up packages")
			return
		}
		if pkg.MinTopupMicros > 0 && payload.AmountMicros < pkg.MinTopupMicros {
			writeError(w, http.StatusBadRequest, "amount is below the minimum top-up")
			return
		}
		if pkg.MaxTopupMicros > 0 && payload.AmountMicros > pkg.MaxTopupMicros {
			writeError(w, http.StatusBadRequest, "amount is above the maximum top-up")
			return
		}
		priceMicros = payload.AmountMicros
	}
	// 手续费：enabled 且 订单金额 < 阈值 时加收（admin 通过 global-vars 配置）。
	feeMicros := int64(0)
	if surchargeEnabled, feeMicrosCfg, thresholdMicros := r.loadPaymentSurcharge(req.Context()); surchargeEnabled && thresholdMicros > 0 && priceMicros < thresholdMicros {
		feeMicros = feeMicrosCfg
	}
	totalMicros := priceMicros + feeMicros
	amountMinor, err := microsToCurrencyMinor(totalMicros)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	feeMinor := int64(0)
	if feeMicros > 0 {
		feeMinor, _ = microsToCurrencyMinor(feeMicros)
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
		slog.Error("stripe checkout session creation failed", "user_id", authUser.ID, "tier_code", pkg.Code, "error", err)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to create stripe checkout session: %v", err))
		return
	}
	if err := r.recordCheckoutSession(req.Context(), "stripe", session.ID, authUser.ID, pkg, customerEmail, amountMinor, r.stripeClient.Currency(), feeMinor); err != nil {
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

func (r *routes) handleAdminQuickCreateUser(w http.ResponseWriter, req *http.Request) {
	r.handleQuickCreateUser(w, req, 0)
}

func (r *routes) handleDistributorQuickCreateUser(w http.ResponseWriter, req *http.Request) {
	authUser, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	r.handleQuickCreateUser(w, req, authUser.ID)
}

func (r *routes) handleQuickCreateUser(w http.ResponseWriter, req *http.Request, distributorUserID int64) {
	if r.proxyClient == nil {
		writeError(w, http.StatusInternalServerError, "auth proxy is not configured")
		return
	}

	var payload adminQuickCreateUserRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.Email = strings.TrimSpace(strings.ToLower(payload.Email))
	if payload.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	if _, found, err := r.findLocalUserIDByEmail(req.Context(), payload.Email); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check user")
		return
	} else if found {
		writeError(w, http.StatusConflict, "email already exists")
		return
	}

	password, err := generateRandomPassword(12)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate password")
		return
	}
	hash, err := user.HashPassword(password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	name := payload.Email
	if atIdx := strings.Index(payload.Email, "@"); atIdx > 0 {
		name = payload.Email[:atIdx]
	}

	upstreamBody, upstreamStatus, err := r.registerSub2APIUser(req.Context(), payload.Email, name, password)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to register upstream user")
		return
	}
	if upstreamStatus < 200 || upstreamStatus >= 300 {
		status := http.StatusBadGateway
		if upstreamStatus == http.StatusConflict {
			status = http.StatusConflict
		}
		writeError(w, status, formatUpstreamRegisterError(upstreamStatus, upstreamBody))
		return
	}

	accessToken, refreshToken, _, upstreamUserID, ok := extractSub2APITokensFromAuthResponse(upstreamBody)
	if !ok || strings.TrimSpace(accessToken) == "" {
		writeError(w, http.StatusBadGateway, "sub2api register response missing access token")
		return
	}
	if upstreamUserID == nil || *upstreamUserID <= 0 {
		writeError(w, http.StatusBadGateway, "sub2api register response missing user id")
		return
	}

	tx, err := r.db.BeginTx(req.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	userID := *upstreamUserID
	if _, err := tx.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `
		INSERT INTO als_users(id, email, name, role, password_hash, email_verified)
		VALUES (?, ?, ?, 'user', ?, TRUE);
	`), userID, payload.Email, name, hash); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, http.StatusConflict, "email already exists")
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to create user: %v", err))
		return
	}

	if _, err := tx.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `
		INSERT INTO als_user_wallets(user_id, balance_micros, currency)
		VALUES (?, 0, 'CNY')
		ON CONFLICT(user_id) DO NOTHING
	`), userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create wallet")
		return
	}

	var refreshTokenValue any
	if refreshToken != nil && strings.TrimSpace(*refreshToken) != "" {
		refreshTokenValue = strings.TrimSpace(*refreshToken)
	}
	if _, err := tx.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `
		INSERT INTO als_sub2api_auth_tokens(
			user_id,
			upstream_user_id,
			access_token,
			refresh_token,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			upstream_user_id = COALESCE(excluded.upstream_user_id, als_sub2api_auth_tokens.upstream_user_id),
			access_token = excluded.access_token,
			refresh_token = COALESCE(excluded.refresh_token, als_sub2api_auth_tokens.refresh_token),
			updated_at = excluded.updated_at
	`), userID, *upstreamUserID, strings.TrimSpace(accessToken), refreshTokenValue, now, now); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store upstream auth token")
		return
	}

	var bindingID int64
	if distributorUserID > 0 {
		if r.sqlDialect == "postgres" {
			if err := tx.QueryRowContext(req.Context(), db.Rebind(r.sqlDialect, `
				INSERT INTO als_distributor_user_bindings(distributor_user_id, user_id, source, created_at, updated_at)
				VALUES (?, ?, 'distributor_quick_create', ?, ?)
				ON CONFLICT(user_id) DO UPDATE SET
					distributor_user_id = excluded.distributor_user_id,
					source = excluded.source,
					updated_at = excluded.updated_at
				RETURNING id;
			`), distributorUserID, userID, now, now).Scan(&bindingID); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to bind distributor user")
				return
			}
		} else {
			if _, err := tx.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `
				INSERT INTO als_distributor_user_bindings(distributor_user_id, user_id, source, created_at, updated_at)
				VALUES (?, ?, 'distributor_quick_create', ?, ?)
				ON CONFLICT(user_id) DO UPDATE SET
					distributor_user_id = excluded.distributor_user_id,
					source = excluded.source,
					updated_at = excluded.updated_at;
			`), distributorUserID, userID, now, now); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to bind distributor user")
				return
			}
			if err := tx.QueryRowContext(req.Context(), db.Rebind(r.sqlDialect, `SELECT id FROM als_distributor_user_bindings WHERE user_id = ?;`), userID).Scan(&bindingID); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to bind distributor user")
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	writeJSON(w, http.StatusCreated, adminQuickCreateUserResponse{
		ID:                   *upstreamUserID,
		DistributorBindingID: bindingID,
		Email:                payload.Email,
		Name:                 name,
		Password:             password,
		CreatedAt:            now.Format(time.RFC3339),
	})
}

func (r *routes) handleAdminUpdateUserRole(w http.ResponseWriter, req *http.Request) {
	var payload adminUpdateUserRoleRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.Email = strings.TrimSpace(strings.ToLower(payload.Email))
	payload.Role = strings.TrimSpace(strings.ToLower(payload.Role))
	if !isSupportedLocalRole(payload.Role) {
		writeError(w, http.StatusBadRequest, "role must be user, admin, or distributor")
		return
	}
	if payload.UserID <= 0 && payload.Email == "" {
		writeError(w, http.StatusBadRequest, "user_id or email is required")
		return
	}

	target, err := r.resolveLocalUserForRoleUpdate(req.Context(), payload.UserID, payload.Email)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}

	if target.Role == "admin" && payload.Role != "admin" {
		hasOtherAdmin, err := r.hasOtherLocalAdmin(req.Context(), target.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to validate admin role change")
			return
		}
		if !hasOtherAdmin {
			writeError(w, http.StatusConflict, "cannot remove the last admin role")
			return
		}
	}

	now := time.Now().UTC()
	result, err := r.db.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `
		UPDATE als_users
		SET role = ?, updated_at = ?
		WHERE id = ?;
	`), payload.Role, now, target.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user role")
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user role")
		return
	}
	if affected == 0 {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	responseID := target.ID
	if target.UpstreamUserID != nil && *target.UpstreamUserID > 0 {
		responseID = *target.UpstreamUserID
	}
	writeJSON(w, http.StatusOK, adminUpdateUserRoleResponse{
		ID:            responseID,
		Sub2APIUserID: target.UpstreamUserID,
		Email:         target.Email,
		Name:          target.Name,
		Role:          payload.Role,
		UpdatedAt:     now.Format(time.RFC3339),
	})
}

func (r *routes) handleAdminBindDistributorUser(w http.ResponseWriter, req *http.Request) {
	var payload adminBindDistributorUserRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.DistributorEmail = strings.TrimSpace(strings.ToLower(payload.DistributorEmail))
	payload.Email = strings.TrimSpace(strings.ToLower(payload.Email))
	payload.Source = strings.TrimSpace(payload.Source)
	if payload.Source == "" {
		payload.Source = "manual"
	}

	distributor, err := r.resolveLocalUserForRoleUpdate(req.Context(), payload.DistributorUserID, payload.DistributorEmail)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "distributor not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load distributor")
		return
	}
	if distributor.Role != "distributor" {
		writeError(w, http.StatusBadRequest, "target distributor user must have distributor role")
		return
	}

	target, err := r.resolveLocalUserForRoleUpdate(req.Context(), payload.UserID, payload.Email)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	if target.Role == "admin" || target.Role == "distributor" {
		writeError(w, http.StatusBadRequest, "only normal users can be bound to a distributor")
		return
	}

	now := time.Now().UTC()
	bindingID, err := r.upsertDistributorUserBinding(req.Context(), distributor.ID, target.ID, payload.Source, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to bind distributor user")
		return
	}

	writeJSON(w, http.StatusOK, distributorUserBindingResponse{
		ID:                bindingID,
		DistributorUserID: distributor.ID,
		UserID:            target.ID,
		Email:             target.Email,
		Name:              target.Name,
		Source:            payload.Source,
		CreatedAt:         now.Format(time.RFC3339),
	})
}

func (r *routes) handleAdminListDistributorInvitations(w http.ResponseWriter, req *http.Request) {
	paging, err := parseListPagination(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	invitations, pagination, err := r.listDistributorInvitations(req.Context(), 0, paging)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list distributor invitations")
		return
	}
	writeJSON(w, http.StatusOK, listDistributorInvitationsResponse{Invitations: invitations, Pagination: pagination})
}

func (r *routes) handleAdminDistributorAssignmentStats(w http.ResponseWriter, req *http.Request) {
	distributorUserID, err := parseOptionalPositiveInt64(req.URL.Query().Get("distributor_user_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "distributor_user_id must be a positive integer")
		return
	}
	from, to, err := parseStatsDateRange(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	stats, err := r.loadDistributorAssignmentStats(req.Context(), distributorUserID, from, to, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load distributor assignment stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (r *routes) handleDistributorListUsers(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	paging, err := parseListPagination(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	users, pagination, err := r.listDistributorUsers(req.Context(), user.ID, paging)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list distributor users")
		return
	}
	writeJSON(w, http.StatusOK, listDistributorUsersResponse{Users: users, Pagination: pagination})
}

func (r *routes) handleDistributorListInvitations(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	paging, err := parseListPagination(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	invitations, pagination, err := r.listDistributorInvitations(req.Context(), user.ID, paging)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list distributor invitations")
		return
	}
	writeJSON(w, http.StatusOK, listDistributorInvitationsResponse{Invitations: invitations, Pagination: pagination})
}

func (r *routes) handleDistributorListPackages(w http.ResponseWriter, req *http.Request) {
	packages, err := r.listAdminPackages(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list packages")
		return
	}
	packages = filterDistributorAssignablePackages(packages)
	writeJSON(w, http.StatusOK, listAdminPackagesResponse{Packages: packages})
}

func (r *routes) handleDistributorAssignmentStats(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	from, to, err := parseStatsDateRange(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	stats, err := r.loadDistributorAssignmentStats(req.Context(), user.ID, from, to, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load distributor assignment stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (r *routes) handleAdminAssignPackage(w http.ResponseWriter, req *http.Request) {
	if r.fulfillmentSvc == nil {
		writeError(w, http.StatusInternalServerError, "fulfillment service is not configured")
		return
	}

	var payload adminAssignPackageRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.Email = strings.TrimSpace(strings.ToLower(payload.Email))
	payload.TierCode = strings.TrimSpace(payload.TierCode)
	payload.Password = strings.TrimSpace(payload.Password)
	if payload.UserID <= 0 && payload.Email == "" {
		writeError(w, http.StatusBadRequest, "user_id or email is required")
		return
	}
	if payload.TierCode == "" {
		writeError(w, http.StatusBadRequest, "tier_code is required")
		return
	}

	localUserID, upstreamUserID, err := r.resolveAdminPackageTargetUser(req.Context(), payload.UserID, payload.Email)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check user")
		return
	}
	authUser, hasAuthUser := auth.UserFromContext(req.Context())
	if hasAuthUser && authUser.Role == "distributor" {
		allowed, err := r.isUserBoundToDistributor(req.Context(), authUser.ID, localUserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check distributor user")
			return
		}
		if !allowed {
			writeError(w, http.StatusForbidden, "distributor can only assign packages to bound users")
			return
		}
	}
	if payload.Password != "" {
		profile, profileErr := r.userSvc.GetProfile(req.Context(), localUserID)
		if profileErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to load user profile")
			return
		}
		if err := r.syncSub2APIAuthTokensWithPassword(req.Context(), localUserID, profile.Email, payload.Password); err != nil {
			writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to refresh sub2api token with provided password: %v", err))
			return
		}
	}

	pkg, err := r.loadAdminPackageByCode(req.Context(), payload.TierCode)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("package not found: %v", err))
		return
	}
	if hasAuthUser {
		if authUser.Role == "distributor" && !isDistributorAssignablePackage(pkg) {
			writeError(w, http.StatusForbidden, "this role can only assign available distributor packages")
			return
		}
	}
	if !pkg.IsPublished {
		writeError(w, http.StatusBadRequest, "package is not published")
		return
	}
	if err := r.validatePackageGroupBindings(req.Context(), adminPackageRequest{
		GroupIDs:    pkg.GroupIDs,
		ValueType:   pkg.ValueType,
		ValueAmount: pkg.ValueAmount,
	}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	paymentEventID := fmt.Sprintf("admin-assign-%d-%d", upstreamUserID, time.Now().UTC().UnixMilli())
	orderID := fmt.Sprintf("admin-order-%d", time.Now().UTC().UnixMilli())
	idempotencyKey := fmt.Sprintf("admin-assign-%s-%d-%d", payload.TierCode, upstreamUserID, time.Now().UTC().UnixNano())

	checkoutSessionID := fmt.Sprintf("cs_admin_%d_%d", upstreamUserID, time.Now().UTC().UnixMilli())
	if recordErr := r.recordCheckoutSession(req.Context(), "admin", checkoutSessionID, localUserID, pkg, "", pkg.PriceMicros, "cny", 0); recordErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to record checkout session")
		return
	}

	assignPayload := adminPaymentSuccessRequest{
		PaymentEventID: paymentEventID,
		OrderID:        orderID,
		Provider:       "admin",
		UserID:         localUserID,
		TierCode:       payload.TierCode,
	}

	job, err := r.ingestAndMaybeExecutePaymentSuccess(req.Context(), assignPayload, idempotencyKey)
	if err != nil {
		if errors.Is(err, fulfillment.ErrIdempotencyConflict) {
			writeError(w, http.StatusConflict, "idempotency conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("fulfillment failed: %v", err))
		return
	}

	if job != nil && job.ID > 0 {
		_ = r.markCheckoutSessionCompleted(req.Context(), "admin", checkoutSessionID, paymentEventID, payload.TierCode, localUserID, "", pkg.PriceMicros, "cny", job.ID, nil)
	}
	if hasAuthUser && authUser.Role == "distributor" {
		status := "created"
		var fulfillmentJobID *int64
		if job != nil {
			status = job.Status
			if job.ID > 0 {
				fulfillmentJobID = &job.ID
			}
		}
		_ = r.recordDistributorPackageAssignment(req.Context(), authUser.ID, localUserID, payload.TierCode, pkg.PriceMicros, fulfillmentJobID, status)
	}

	var jobResp *fulfillmentJobResponse
	if job != nil {
		resp := toFulfillmentJobResponse(job)
		jobResp = &resp
	}
	if job != nil {
		switch job.Status {
		case fulfillment.StatusFailedTerminal:
			writeJSON(w, http.StatusBadRequest, adminAssignPackageResponse{
				PaymentEventID: paymentEventID,
				TierCode:       payload.TierCode,
				FulfillmentJob: jobResp,
			})
			return
		case fulfillment.StatusFailedRetryable:
			writeJSON(w, http.StatusBadGateway, adminAssignPackageResponse{
				PaymentEventID: paymentEventID,
				TierCode:       payload.TierCode,
				FulfillmentJob: jobResp,
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, adminAssignPackageResponse{
		PaymentEventID: paymentEventID,
		TierCode:       payload.TierCode,
		FulfillmentJob: jobResp,
	})
}

func generateRandomPassword(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("password length must be positive")
	}
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b)[:length], nil
}

func (r *routes) registerSub2APIUser(ctx context.Context, email, name, password string) ([]byte, int, error) {
	bodyBytes, err := json.Marshal(map[string]string{
		"email":    strings.TrimSpace(email),
		"name":     strings.TrimSpace(name),
		"password": password,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("marshal upstream register payload: %w", err)
	}

	forwarded, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://portal.local/auth/register", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("build upstream register request: %w", err)
	}
	forwarded.Header.Set("Content-Type", "application/json")
	forwarded.Header.Set("Accept", "application/json")

	resp, err := r.proxyClient.Do(ctx, forwarded, "/api/v1/auth/register")
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read upstream register response: %w", err)
	}
	return responseBody, resp.StatusCode, nil
}

func (r *routes) loginSub2APIUser(ctx context.Context, email, password string) ([]byte, int, error) {
	bodyBytes, err := json.Marshal(map[string]string{
		"email":    strings.TrimSpace(email),
		"password": password,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("marshal upstream login payload: %w", err)
	}

	forwarded, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://portal.local/auth/login", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("build upstream login request: %w", err)
	}
	forwarded.Header.Set("Content-Type", "application/json")
	forwarded.Header.Set("Accept", "application/json")

	resp, err := r.proxyClient.Do(ctx, forwarded, "/api/v1/auth/login")
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read upstream login response: %w", err)
	}
	return responseBody, resp.StatusCode, nil
}

func (r *routes) trySyncSub2APIAuthTokensWithPassword(ctx context.Context, userID int64, email, password, reason string) {
	if err := r.syncSub2APIAuthTokensWithPassword(ctx, userID, email, password); err != nil {
		slog.Warn("sub2api token sync with password failed", "user_id", userID, "email", strings.TrimSpace(email), "reason", reason, "error", err)
	}
}

func (r *routes) resolveLocalUserForRoleUpdate(ctx context.Context, upstreamOrLocalUserID int64, email string) (localUserRoleTarget, error) {
	trimmedEmail := strings.TrimSpace(email)
	if upstreamOrLocalUserID > 0 {
		if localUserID, found, err := r.findLocalUserIDByUpstreamUserID(ctx, upstreamOrLocalUserID); err != nil {
			return localUserRoleTarget{}, err
		} else if found {
			target, err := r.loadLocalUserForRoleUpdate(ctx, localUserID, "")
			if err != nil {
				return localUserRoleTarget{}, err
			}
			target.UpstreamUserID = &upstreamOrLocalUserID
			return target, nil
		}

		if r.proxyClient != nil {
			localUserID, importErr := r.importSub2APIUserForAdminPackage(ctx, upstreamOrLocalUserID)
			if importErr == nil {
				target, err := r.loadLocalUserForRoleUpdate(ctx, localUserID, "")
				if err != nil {
					return localUserRoleTarget{}, err
				}
				target.UpstreamUserID = &upstreamOrLocalUserID
				return target, nil
			}
			if !errors.Is(importErr, sql.ErrNoRows) {
				return localUserRoleTarget{}, importErr
			}
		}

		// Backward-compatible escape hatch for local-only bootstrap/admin users.
		return r.loadLocalUserForRoleUpdate(ctx, upstreamOrLocalUserID, "")
	}

	if trimmedEmail == "" {
		return localUserRoleTarget{}, sql.ErrNoRows
	}
	target, err := r.loadLocalUserForRoleUpdate(ctx, 0, trimmedEmail)
	if err != nil {
		return localUserRoleTarget{}, err
	}
	if upstreamUserID, found, err := r.sub2api.UpstreamUserID(ctx, target.ID); err == nil && found {
		target.UpstreamUserID = &upstreamUserID
	}
	return target, nil
}

func (r *routes) syncSub2APIAuthTokensWithPassword(ctx context.Context, userID int64, email, password string) error {
	if r.proxyClient == nil || !r.sub2api.IsConfigured() {
		return nil
	}
	email = strings.TrimSpace(strings.ToLower(email))
	password = strings.TrimSpace(password)
	if userID <= 0 || email == "" || password == "" {
		return nil
	}

	body, status, err := r.loginSub2APIUser(ctx, email, password)
	if err != nil {
		return fmt.Errorf("login upstream user: %w", err)
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("sub2api login failed: %s", formatUpstreamAuthError(status, body))
	}

	accessToken, refreshToken, _, upstreamUserID, ok := extractSub2APITokensFromAuthResponse(body)
	if !ok || strings.TrimSpace(accessToken) == "" {
		return errors.New("sub2api login response missing access token")
	}

	if err := r.sub2api.CaptureTokens(ctx, sub2apiauth.UpsertTokenInput{
		UserID:          userID,
		UpstreamUserID:  upstreamUserID,
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		AccessExpiresAt: accessExpiryOrDefault(accessToken),
	}); err != nil {
		return err
	}
	return nil
}

func (r *routes) resolveAdminPackageUserIDs(ctx context.Context, requestedUserID int64) (int64, int64, error) {
	if requestedUserID <= 0 {
		return 0, 0, errors.New("user id must be positive")
	}

	localUserID, found, err := r.findLocalUserIDByUpstreamUserID(ctx, requestedUserID)
	if err != nil {
		return 0, 0, err
	}
	if found {
		return localUserID, requestedUserID, nil
	}

	var exists int64
	err = r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `SELECT 1 FROM als_users WHERE id = ?`), requestedUserID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		localUserID, importErr := r.importSub2APIUserForAdminPackage(ctx, requestedUserID)
		if importErr != nil {
			return 0, 0, importErr
		}
		return localUserID, requestedUserID, nil
	}
	if err != nil {
		return 0, 0, err
	}

	upstreamUserID, err := r.resolveSub2APIUserID(ctx, requestedUserID)
	if err != nil {
		return 0, 0, err
	}
	return requestedUserID, upstreamUserID, nil
}

func (r *routes) resolveAdminPackageTargetUser(ctx context.Context, requestedUserID int64, email string) (int64, int64, error) {
	if requestedUserID > 0 {
		return r.resolveAdminPackageUserIDs(ctx, requestedUserID)
	}

	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail == "" {
		return 0, 0, sql.ErrNoRows
	}

	localUserID, found, err := r.findLocalUserIDByEmail(ctx, trimmedEmail)
	if err != nil {
		return 0, 0, err
	}
	if !found {
		return 0, 0, sql.ErrNoRows
	}

	upstreamUserID, err := r.resolveSub2APIUserID(ctx, localUserID)
	if err != nil {
		return 0, 0, err
	}
	return localUserID, upstreamUserID, nil
}

func (r *routes) upsertDistributorUserBinding(ctx context.Context, distributorUserID, targetUserID int64, source string, now time.Time) (int64, error) {
	if r.sqlDialect == "postgres" {
		var id int64
		err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
			INSERT INTO als_distributor_user_bindings(distributor_user_id, user_id, source, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(user_id) DO UPDATE SET
				distributor_user_id = excluded.distributor_user_id,
				source = excluded.source,
				updated_at = excluded.updated_at
			RETURNING id;
		`), distributorUserID, targetUserID, source, now, now).Scan(&id)
		return id, err
	}

	if _, err := r.db.ExecContext(ctx, db.Rebind(r.sqlDialect, `
		INSERT INTO als_distributor_user_bindings(distributor_user_id, user_id, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			distributor_user_id = excluded.distributor_user_id,
			source = excluded.source,
			updated_at = excluded.updated_at;
	`), distributorUserID, targetUserID, source, now, now); err != nil {
		return 0, err
	}

	var id int64
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `SELECT id FROM als_distributor_user_bindings WHERE user_id = ?;`), targetUserID).Scan(&id)
	return id, err
}

func (r *routes) isUserBoundToDistributor(ctx context.Context, distributorUserID, targetUserID int64) (bool, error) {
	var exists int
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT 1
		FROM als_distributor_user_bindings
		WHERE distributor_user_id = ?
			AND user_id = ?
		LIMIT 1;
	`), distributorUserID, targetUserID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *routes) recordDistributorPackageAssignment(ctx context.Context, distributorUserID, targetUserID int64, tierCode string, priceMicros int64, fulfillmentJobID *int64, status string) error {
	var jobArg any
	if fulfillmentJobID != nil {
		jobArg = *fulfillmentJobID
	}
	_, err := r.db.ExecContext(ctx, db.Rebind(r.sqlDialect, `
		INSERT INTO als_distributor_package_assignments(distributor_user_id, target_user_id, tier_code, price_micros, fulfillment_job_id, status)
		VALUES (?, ?, ?, ?, ?, ?);
	`), distributorUserID, targetUserID, tierCode, priceMicros, jobArg, strings.TrimSpace(status))
	return err
}

func parseOptionalPositiveInt64(value string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid positive integer")
	}
	return parsed, nil
}

func parseListPagination(req *http.Request) (listPaginationRequest, error) {
	const (
		defaultPerPage = 20
		maxPerPage     = 100
	)
	query := req.URL.Query()
	page := 1
	if raw := strings.TrimSpace(query.Get("page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return listPaginationRequest{}, errors.New("page must be a positive integer")
		}
		page = parsed
	}
	perPage := defaultPerPage
	if raw := strings.TrimSpace(query.Get("per_page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return listPaginationRequest{}, errors.New("per_page must be a positive integer")
		}
		perPage = parsed
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	return listPaginationRequest{
		Page:    page,
		PerPage: perPage,
		Offset:  (page - 1) * perPage,
	}, nil
}

func buildPaginationResponse(paging listPaginationRequest, total int64) paginationResponse {
	if paging.Page <= 0 {
		paging.Page = 1
	}
	if paging.PerPage <= 0 {
		paging.PerPage = 20
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(paging.PerPage) - 1) / int64(paging.PerPage))
	}
	return paginationResponse{
		Page:       paging.Page,
		PerPage:    paging.PerPage,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    totalPages > 0 && paging.Page < totalPages,
		HasPrev:    paging.Page > 1,
	}
}

func parseStatsDateRange(req *http.Request) (*time.Time, *time.Time, error) {
	parseDate := func(name string) (*time.Time, error) {
		value := strings.TrimSpace(req.URL.Query().Get(name))
		if value == "" {
			return nil, nil
		}
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil {
			return nil, fmt.Errorf("%s must use YYYY-MM-DD", name)
		}
		return &parsed, nil
	}

	from, err := parseDate("from")
	if err != nil {
		return nil, nil, err
	}
	to, err := parseDate("to")
	if err != nil {
		return nil, nil, err
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, nil, errors.New("from must be before or equal to to")
	}
	return from, to, nil
}

func assignmentDateExpr(dialect string) string {
	if dialect == "postgres" {
		return "DATE(dpa.created_at)::text"
	}
	return "DATE(dpa.created_at)"
}

func assignmentLatestExpr(dialect string) string {
	if dialect == "postgres" {
		return "MAX(dpa.created_at)::text"
	}
	return "MAX(dpa.created_at)"
}

func assignmentPriceExpr() string {
	return "COALESCE(NULLIF(dpa.price_micros, 0), t.price_micros, 0)"
}

func addAssignmentStatsFilters(args []any, distributorUserID int64, from, to *time.Time) (string, []any) {
	clauses := []string{"1 = 1"}
	if distributorUserID > 0 {
		clauses = append(clauses, "dpa.distributor_user_id = ?")
		args = append(args, distributorUserID)
	}
	if from != nil {
		clauses = append(clauses, "dpa.created_at >= ?")
		args = append(args, *from)
	}
	if to != nil {
		clauses = append(clauses, "dpa.created_at < ?")
		args = append(args, to.AddDate(0, 0, 1))
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (r *routes) loadDistributorAssignmentStats(ctx context.Context, distributorUserID int64, from, to *time.Time, includeDistributorBreakdown bool) (distributorAssignmentStatsResponse, error) {
	var response distributorAssignmentStatsResponse
	whereSQL, args := addAssignmentStatsFilters(nil, distributorUserID, from, to)
	priceExpr := assignmentPriceExpr()

	totalsQuery := fmt.Sprintf(`
		SELECT
			COUNT(*),
			COUNT(DISTINCT dpa.target_user_id),
			COUNT(DISTINCT dpa.distributor_user_id),
			COALESCE(SUM(%s), 0)
		FROM als_distributor_package_assignments dpa
		LEFT JOIN als_tiers t ON t.code = dpa.tier_code
		%s;
	`, priceExpr, whereSQL)
	if err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, totalsQuery), args...).Scan(
		&response.Totals.AssignmentCount,
		&response.Totals.UniqueUserCount,
		&response.Totals.DistributorCount,
		&response.Totals.TotalPriceMicros,
	); err != nil {
		return distributorAssignmentStatsResponse{}, err
	}

	daily, err := r.loadDistributorAssignmentDailyStats(ctx, whereSQL, args, priceExpr)
	if err != nil {
		return distributorAssignmentStatsResponse{}, err
	}
	response.Daily = daily

	packages, err := r.loadDistributorAssignmentPackageStats(ctx, whereSQL, args, priceExpr)
	if err != nil {
		return distributorAssignmentStatsResponse{}, err
	}
	response.Packages = packages

	users, err := r.loadDistributorAssignmentUserStats(ctx, whereSQL, args, priceExpr)
	if err != nil {
		return distributorAssignmentStatsResponse{}, err
	}
	response.Users = users

	if includeDistributorBreakdown {
		distributors, err := r.loadDistributorAssignmentDistributorStats(ctx, whereSQL, args, priceExpr)
		if err != nil {
			return distributorAssignmentStatsResponse{}, err
		}
		response.Distributors = distributors
	}
	return response, nil
}

func (r *routes) loadDistributorAssignmentDailyStats(ctx context.Context, whereSQL string, args []any, priceExpr string) ([]distributorAssignmentDailyStatsResponse, error) {
	dateExpr := assignmentDateExpr(r.sqlDialect)
	query := fmt.Sprintf(`
		SELECT
			%s AS assigned_date,
			COUNT(*),
			COALESCE(SUM(%s), 0)
		FROM als_distributor_package_assignments dpa
		LEFT JOIN als_tiers t ON t.code = dpa.tier_code
		%s
		GROUP BY assigned_date
		ORDER BY assigned_date DESC
		LIMIT 90;
	`, dateExpr, priceExpr, whereSQL)

	rows, err := r.db.QueryContext(ctx, db.Rebind(r.sqlDialect, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]distributorAssignmentDailyStatsResponse, 0)
	for rows.Next() {
		var item distributorAssignmentDailyStatsResponse
		if err := rows.Scan(&item.Date, &item.AssignmentCount, &item.TotalPriceMicros); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *routes) loadDistributorAssignmentPackageStats(ctx context.Context, whereSQL string, args []any, priceExpr string) ([]distributorAssignmentPackageStatsResponse, error) {
	latestExpr := assignmentLatestExpr(r.sqlDialect)
	query := fmt.Sprintf(`
		SELECT
			dpa.tier_code,
			COALESCE(MAX(t.name), ''),
			COUNT(*),
			COALESCE(SUM(%s), 0),
			%s
		FROM als_distributor_package_assignments dpa
		LEFT JOIN als_tiers t ON t.code = dpa.tier_code
		%s
		GROUP BY dpa.tier_code
		ORDER BY COUNT(*) DESC, dpa.tier_code ASC
		LIMIT 100;
	`, priceExpr, latestExpr, whereSQL)

	rows, err := r.db.QueryContext(ctx, db.Rebind(r.sqlDialect, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]distributorAssignmentPackageStatsResponse, 0)
	for rows.Next() {
		var item distributorAssignmentPackageStatsResponse
		var latest sql.NullString
		if err := rows.Scan(&item.TierCode, &item.PackageName, &item.AssignmentCount, &item.TotalPriceMicros, &latest); err != nil {
			return nil, err
		}
		if latest.Valid {
			item.LatestAssignedAt = latest.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *routes) loadDistributorAssignmentUserStats(ctx context.Context, whereSQL string, args []any, priceExpr string) ([]distributorAssignmentUserStatsResponse, error) {
	latestExpr := assignmentLatestExpr(r.sqlDialect)
	query := fmt.Sprintf(`
		SELECT
			u.id,
			u.email,
			u.name,
			COUNT(*),
			COALESCE(SUM(%s), 0),
			%s
		FROM als_distributor_package_assignments dpa
		JOIN als_users u ON u.id = dpa.target_user_id
		LEFT JOIN als_tiers t ON t.code = dpa.tier_code
		%s
		GROUP BY u.id, u.email, u.name
		ORDER BY COUNT(*) DESC, u.id ASC
		LIMIT 100;
	`, priceExpr, latestExpr, whereSQL)

	rows, err := r.db.QueryContext(ctx, db.Rebind(r.sqlDialect, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]distributorAssignmentUserStatsResponse, 0)
	for rows.Next() {
		var item distributorAssignmentUserStatsResponse
		var latest sql.NullString
		if err := rows.Scan(&item.UserID, &item.Email, &item.Name, &item.AssignmentCount, &item.TotalPriceMicros, &latest); err != nil {
			return nil, err
		}
		if latest.Valid {
			item.LatestAssignedAt = latest.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *routes) loadDistributorAssignmentDistributorStats(ctx context.Context, whereSQL string, args []any, priceExpr string) ([]distributorAssignmentDistributorStatsResponse, error) {
	latestExpr := assignmentLatestExpr(r.sqlDialect)
	query := fmt.Sprintf(`
		SELECT
			du.id,
			du.email,
			du.name,
			COUNT(*),
			COUNT(DISTINCT dpa.target_user_id),
			COALESCE(SUM(%s), 0),
			%s
		FROM als_distributor_package_assignments dpa
		JOIN als_users du ON du.id = dpa.distributor_user_id
		LEFT JOIN als_tiers t ON t.code = dpa.tier_code
		%s
		GROUP BY du.id, du.email, du.name
		ORDER BY COUNT(*) DESC, du.id ASC
		LIMIT 100;
	`, priceExpr, latestExpr, whereSQL)

	rows, err := r.db.QueryContext(ctx, db.Rebind(r.sqlDialect, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]distributorAssignmentDistributorStatsResponse, 0)
	for rows.Next() {
		var item distributorAssignmentDistributorStatsResponse
		var latest sql.NullString
		if err := rows.Scan(
			&item.DistributorUserID,
			&item.DistributorEmail,
			&item.DistributorName,
			&item.AssignmentCount,
			&item.UniqueUserCount,
			&item.TotalPriceMicros,
			&latest,
		); err != nil {
			return nil, err
		}
		if latest.Valid {
			item.LatestAssignedAt = latest.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *routes) listDistributorInvitations(ctx context.Context, distributorUserID int64, paging listPaginationRequest) ([]distributorInvitationResponse, paginationResponse, error) {
	countQuery := `SELECT COUNT(*) FROM als_distributor_user_bindings dub`
	args := []any{}
	if distributorUserID > 0 {
		countQuery += " WHERE dub.distributor_user_id = ?"
		args = append(args, distributorUserID)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, countQuery), args...).Scan(&total); err != nil {
		return nil, paginationResponse{}, err
	}

	query := `
		SELECT
			dub.id,
			dub.distributor_user_id,
			du.email,
			du.name,
			u.id,
			u.email,
			u.name,
			dub.source,
			dub.created_at,
			dub.updated_at,
			sat.upstream_user_id
		FROM als_distributor_user_bindings dub
		JOIN als_users du ON du.id = dub.distributor_user_id
		JOIN als_users u ON u.id = dub.user_id
		LEFT JOIN als_sub2api_auth_tokens sat ON sat.user_id = u.id
	`
	queryArgs := append([]any(nil), args...)
	if distributorUserID > 0 {
		query += " WHERE dub.distributor_user_id = ?"
	}
	query += " ORDER BY dub.created_at DESC, dub.id DESC LIMIT ? OFFSET ?;"
	queryArgs = append(queryArgs, paging.PerPage, paging.Offset)

	rows, err := r.db.QueryContext(ctx, db.Rebind(r.sqlDialect, query), queryArgs...)
	if err != nil {
		return nil, paginationResponse{}, err
	}
	defer rows.Close()

	invitations := make([]distributorInvitationResponse, 0)
	for rows.Next() {
		var item distributorInvitationResponse
		var createdAt time.Time
		var updatedAt time.Time
		if err := rows.Scan(
			&item.ID,
			&item.DistributorUserID,
			&item.DistributorEmail,
			&item.DistributorName,
			&item.UserID,
			&item.Email,
			&item.Name,
			&item.Source,
			&createdAt,
			&updatedAt,
			&item.UpstreamUserID,
		); err != nil {
			return nil, paginationResponse{}, err
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		item.UpdatedAt = updatedAt.Format(time.RFC3339)
		invitations = append(invitations, item)
	}
	if err := rows.Err(); err != nil {
		return nil, paginationResponse{}, err
	}
	return invitations, buildPaginationResponse(paging, total), nil
}

func (r *routes) listDistributorUsers(ctx context.Context, distributorUserID int64, paging listPaginationRequest) ([]distributorUserSummaryResponse, paginationResponse, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT COUNT(*)
		FROM als_distributor_user_bindings rub
		WHERE rub.distributor_user_id = ?;
	`), distributorUserID).Scan(&total); err != nil {
		return nil, paginationResponse{}, err
	}

	rows, err := r.db.QueryContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT
			u.id,
			sat.upstream_user_id,
			u.email,
			u.name,
			COALESCE(t.code, ''),
			COALESCE(t.name, ''),
			COALESCE(s.status, '')
		FROM als_distributor_user_bindings rub
		JOIN als_users u ON u.id = rub.user_id
		LEFT JOIN als_sub2api_auth_tokens sat ON sat.user_id = u.id
		LEFT JOIN als_subscriptions s ON s.user_id = u.id AND s.status = 'active' AND s.ended_at IS NULL
		LEFT JOIN als_tiers t ON t.id = s.tier_id
		WHERE rub.distributor_user_id = ?
		ORDER BY u.id ASC
		LIMIT ? OFFSET ?;
	`), distributorUserID, paging.PerPage, paging.Offset)
	if err != nil {
		return nil, paginationResponse{}, err
	}
	defer rows.Close()

	users := make([]distributorUserSummaryResponse, 0)
	for rows.Next() {
		var item distributorUserSummaryResponse
		if err := rows.Scan(
			&item.UserID,
			&item.UpstreamUserID,
			&item.Email,
			&item.Name,
			&item.PackageCode,
			&item.PackageName,
			&item.SubscriptionStatus,
		); err != nil {
			return nil, paginationResponse{}, err
		}
		usage, err := r.loadDistributorUserUsageSummary(ctx, item.UserID, "all")
		if err != nil {
			return nil, paginationResponse{}, err
		}
		item.TotalTokens = usage.TotalTokens
		item.ActiveDays = usage.ActiveDays
		item.ActualCostMicros = usage.ActualCostMicros
		item.LastActiveDate = usage.LastActiveDate
		item.UsageSyncedAt = usage.UsageSyncedAt
		item.UsageSource = usage.UsageSource
		item.UsageStale = usage.UsageStale
		item.UsageUnavailable = usage.UsageUnavailable
		users = append(users, item)
	}
	if err := rows.Err(); err != nil {
		return nil, paginationResponse{}, err
	}
	return users, buildPaginationResponse(paging, total), nil
}

const userUsageCacheTTL = 60 * time.Second

type distributorUserUsageCachePayload struct {
	RequestCount     int64  `json:"request_count"`
	InputTokens      int64  `json:"input_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
	ActualCostMicros int64  `json:"actual_cost_micros"`
	ActiveDays       int64  `json:"active_days"`
	LastActiveDate   string `json:"last_active_date,omitempty"`
	SyncedAt         string `json:"synced_at,omitempty"`
	Source           string `json:"source,omitempty"`
	Stale            bool   `json:"stale,omitempty"`
	Unavailable      bool   `json:"unavailable,omitempty"`
}

func (r *routes) loadDistributorUserUsageSummary(ctx context.Context, userID int64, rangeKey string) (distributorUserSummaryResponse, error) {
	rangeKey = normalizeUsageRangeKey(rangeKey)
	if cached, found := r.loadDistributorUsageFromCache(ctx, userID, rangeKey); found {
		return distributorUsagePayloadToResponse(cached, "redis", cached.Stale, cached.Unavailable), nil
	}

	if snapshot, found, err := r.loadDistributorUsageSnapshot(ctx, userID, rangeKey); err != nil {
		return distributorUserSummaryResponse{}, err
	} else if found && isUsageSnapshotFresh(snapshot.SyncedAt) {
		r.storeDistributorUsageInCache(ctx, userID, rangeKey, snapshot)
		return distributorUsagePayloadToResponse(snapshot, "snapshot", false, false), nil
	}

	if usage, err := r.fetchDistributorUsageFromSub2API(ctx, userID); err == nil {
		usage.Source = "sub2api"
		if usage.SyncedAt == "" {
			usage.SyncedAt = time.Now().UTC().Format(time.RFC3339)
		}
		_ = r.upsertDistributorUsageSnapshot(ctx, userID, rangeKey, usage)
		r.storeDistributorUsageInCache(ctx, userID, rangeKey, usage)
		return distributorUsagePayloadToResponse(usage, "sub2api", false, false), nil
	}

	if snapshot, found, err := r.loadDistributorUsageSnapshot(ctx, userID, rangeKey); err != nil {
		return distributorUserSummaryResponse{}, err
	} else if found {
		snapshot.Stale = true
		r.storeDistributorUsageInCache(ctx, userID, rangeKey, snapshot)
		return distributorUsagePayloadToResponse(snapshot, "snapshot", true, false), nil
	}

	local, err := r.loadLocalDistributorUserUsageFallback(ctx, userID)
	if err != nil {
		return distributorUserSummaryResponse{}, err
	}
	if local.TotalTokens > 0 || local.ActiveDays > 0 || local.ActualCostMicros > 0 {
		local.Stale = true
		r.storeDistributorUsageInCache(ctx, userID, rangeKey, local)
		return distributorUsagePayloadToResponse(local, "local_fallback", true, false), nil
	}
	local.Source = "unavailable"
	local.Stale = true
	local.Unavailable = true
	r.storeDistributorUsageInCache(ctx, userID, rangeKey, local)
	return distributorUsagePayloadToResponse(local, "unavailable", true, true), nil
}

func normalizeUsageRangeKey(rangeKey string) string {
	trimmed := strings.TrimSpace(rangeKey)
	if trimmed == "" {
		return "all"
	}
	return trimmed
}

func distributorUsageCacheKey(userID int64, rangeKey string) string {
	return fmt.Sprintf("distributor:user_usage:%d:%s", userID, normalizeUsageRangeKey(rangeKey))
}

func (r *routes) loadDistributorUsageFromCache(ctx context.Context, userID int64, rangeKey string) (distributorUserUsageCachePayload, bool) {
	if r.userUsageCache == nil {
		return distributorUserUsageCachePayload{}, false
	}
	raw, found, err := r.userUsageCache.Get(ctx, distributorUsageCacheKey(userID, rangeKey))
	if err != nil || !found {
		return distributorUserUsageCachePayload{}, false
	}
	var payload distributorUserUsageCachePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return distributorUserUsageCachePayload{}, false
	}
	return payload, true
}

func (r *routes) storeDistributorUsageInCache(ctx context.Context, userID int64, rangeKey string, payload distributorUserUsageCachePayload) {
	if r.userUsageCache == nil {
		return
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = r.userUsageCache.Set(ctx, distributorUsageCacheKey(userID, rangeKey), raw, userUsageCacheTTL)
}

func isUsageSnapshotFresh(syncedAt string) bool {
	parsed, ok := parseUsageSnapshotTime(syncedAt)
	return ok && time.Since(parsed) <= userUsageCacheTTL
}

func parseUsageSnapshotTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z"} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func distributorUsagePayloadToResponse(payload distributorUserUsageCachePayload, source string, stale, unavailable bool) distributorUserSummaryResponse {
	return distributorUserSummaryResponse{
		TotalTokens:      payload.TotalTokens,
		ActiveDays:       payload.ActiveDays,
		ActualCostMicros: payload.ActualCostMicros,
		LastActiveDate:   payload.LastActiveDate,
		UsageSyncedAt:    payload.SyncedAt,
		UsageSource:      source,
		UsageStale:       stale,
		UsageUnavailable: unavailable,
	}
}

func (r *routes) fetchDistributorUsageFromSub2API(ctx context.Context, userID int64) (distributorUserUsageCachePayload, error) {
	if r.proxyClient == nil {
		return distributorUserUsageCachePayload{}, errors.New("sub2api proxy client is not configured")
	}
	upstreamUserID, err := r.resolveSub2APIUserID(ctx, userID)
	if err != nil {
		return distributorUserUsageCachePayload{}, err
	}
	usage, err := r.proxyClient.GetAdminUserUsageSummary(ctx, upstreamUserID)
	if err != nil {
		return distributorUserUsageCachePayload{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return distributorUserUsageCachePayload{
		RequestCount:     usage.RequestCount,
		InputTokens:      usage.InputTokens,
		OutputTokens:     usage.OutputTokens,
		TotalTokens:      usage.TotalTokens,
		ActualCostMicros: usage.ActualCostMicros,
		ActiveDays:       usage.ActiveDays,
		LastActiveDate:   usage.LastActiveDate,
		SyncedAt:         now,
		Source:           "sub2api",
	}, nil
}

func (r *routes) loadDistributorUsageSnapshot(ctx context.Context, userID int64, rangeKey string) (distributorUserUsageCachePayload, bool, error) {
	var payload distributorUserUsageCachePayload
	var syncedAt time.Time
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT
			request_count,
			input_tokens,
			output_tokens,
			total_tokens,
			actual_cost_micros,
			active_days,
			last_active_date,
			source,
			synced_at
		FROM als_distributor_user_usage_snapshots
		WHERE user_id = ?
			AND range_key = ?;
	`), userID, normalizeUsageRangeKey(rangeKey)).Scan(
		&payload.RequestCount,
		&payload.InputTokens,
		&payload.OutputTokens,
		&payload.TotalTokens,
		&payload.ActualCostMicros,
		&payload.ActiveDays,
		&payload.LastActiveDate,
		&payload.Source,
		&syncedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return distributorUserUsageCachePayload{}, false, nil
	}
	if err != nil {
		return distributorUserUsageCachePayload{}, false, err
	}
	payload.SyncedAt = syncedAt.Format(time.RFC3339)
	return payload, true, nil
}

func (r *routes) upsertDistributorUsageSnapshot(ctx context.Context, userID int64, rangeKey string, payload distributorUserUsageCachePayload) error {
	upstreamUserID, err := r.resolveSub2APIUserID(ctx, userID)
	if err != nil {
		upstreamUserID = 0
	}
	now := time.Now().UTC()
	syncedAt := now
	if parsed, ok := parseUsageSnapshotTime(payload.SyncedAt); ok {
		syncedAt = parsed
	}
	if r.sqlDialect == "postgres" {
		_, err = r.db.ExecContext(ctx, db.Rebind(r.sqlDialect, `
			INSERT INTO als_distributor_user_usage_snapshots(
				user_id,
				upstream_user_id,
				range_key,
				request_count,
				input_tokens,
				output_tokens,
				total_tokens,
				actual_cost_micros,
				active_days,
				last_active_date,
				source,
				synced_at,
				created_at,
				updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(user_id, range_key) DO UPDATE SET
				upstream_user_id = excluded.upstream_user_id,
				request_count = excluded.request_count,
				input_tokens = excluded.input_tokens,
				output_tokens = excluded.output_tokens,
				total_tokens = excluded.total_tokens,
				actual_cost_micros = excluded.actual_cost_micros,
				active_days = excluded.active_days,
				last_active_date = excluded.last_active_date,
				source = excluded.source,
				synced_at = excluded.synced_at,
				updated_at = excluded.updated_at;
		`), userID, upstreamUserID, normalizeUsageRangeKey(rangeKey), payload.RequestCount, payload.InputTokens, payload.OutputTokens, payload.TotalTokens, payload.ActualCostMicros, payload.ActiveDays, payload.LastActiveDate, payload.Source, syncedAt, now, now)
		return err
	}
	_, err = r.db.ExecContext(ctx, db.Rebind(r.sqlDialect, `
		INSERT INTO als_distributor_user_usage_snapshots(
			user_id,
			upstream_user_id,
			range_key,
			request_count,
			input_tokens,
			output_tokens,
			total_tokens,
			actual_cost_micros,
			active_days,
			last_active_date,
			source,
			synced_at,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, range_key) DO UPDATE SET
			upstream_user_id = excluded.upstream_user_id,
			request_count = excluded.request_count,
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			total_tokens = excluded.total_tokens,
			actual_cost_micros = excluded.actual_cost_micros,
			active_days = excluded.active_days,
			last_active_date = excluded.last_active_date,
			source = excluded.source,
			synced_at = excluded.synced_at,
			updated_at = excluded.updated_at;
	`), userID, upstreamUserID, normalizeUsageRangeKey(rangeKey), payload.RequestCount, payload.InputTokens, payload.OutputTokens, payload.TotalTokens, payload.ActualCostMicros, payload.ActiveDays, payload.LastActiveDate, payload.Source, syncedAt, now, now)
	return err
}

func (r *routes) loadLocalDistributorUserUsageFallback(ctx context.Context, userID int64) (distributorUserUsageCachePayload, error) {
	var summary distributorUserUsageCachePayload
	var dailyLastActive sql.NullString
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT
			COALESCE(SUM(request_count), 0),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(actual_cost_micros), 0),
			COUNT(DISTINCT CASE WHEN request_count > 0 THEN usage_date END),
			MAX(usage_date)
		FROM als_user_usage_daily
		WHERE user_id = ?;
	`), userID).Scan(&summary.RequestCount, &summary.InputTokens, &summary.OutputTokens, &summary.TotalTokens, &summary.ActualCostMicros, &summary.ActiveDays, &dailyLastActive)
	if err != nil {
		return distributorUserUsageCachePayload{}, err
	}
	if dailyLastActive.Valid {
		summary.LastActiveDate = dailyLastActive.String
	}
	summary.SyncedAt = time.Now().UTC().Format(time.RFC3339)
	summary.Source = "local_fallback"
	return summary, nil
}

func (r *routes) importSub2APIUserForAdminPackage(ctx context.Context, upstreamUserID int64) (int64, error) {
	if upstreamUserID <= 0 {
		return 0, errors.New("upstream user id must be positive")
	}
	if r.proxyClient == nil {
		return 0, errors.New("sub2api proxy client is not configured")
	}

	resp, err := r.proxyClient.GetAdminUser(ctx, upstreamUserID)
	if err != nil {
		var apiErr *proxy.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return 0, sql.ErrNoRows
		}
		return 0, fmt.Errorf("load upstream user %d: %w", upstreamUserID, err)
	}

	upstreamUser := resp.Data
	if upstreamUser.ID > 0 {
		upstreamUserID = upstreamUser.ID
	}

	email := strings.TrimSpace(strings.ToLower(upstreamUser.Email))
	if email == "" {
		email = fmt.Sprintf("sub2api-user-%d@imported.local", upstreamUserID)
	}
	name := strings.TrimSpace(upstreamUser.Name)
	if name == "" {
		name = strings.TrimSpace(upstreamUser.Username)
	}
	if name == "" {
		name = strings.TrimSpace(strings.Split(email, "@")[0])
	}
	if name == "" {
		name = fmt.Sprintf("Sub2API User %d", upstreamUserID)
	}

	localUserID, found, err := r.findLocalUserIDByEmail(ctx, email)
	if err != nil {
		return 0, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin upstream user import tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if !found {
		localUserID = upstreamUserID
		if _, err := tx.ExecContext(ctx, db.Rebind(r.sqlDialect, `
			INSERT INTO als_users(id, email, name, role, email_verified)
			VALUES (?, ?, ?, 'user', TRUE);
		`), localUserID, email, name); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return 0, sql.ErrNoRows
			}
			return 0, fmt.Errorf("create local user for upstream user: %w", err)
		}
	} else if localUserID != upstreamUserID {
		return 0, fmt.Errorf("local user id %d does not match sub2api user id %d for %s", localUserID, upstreamUserID, email)
	}

	if _, err := tx.ExecContext(ctx, db.Rebind(r.sqlDialect, `
		INSERT INTO als_user_wallets(user_id, balance_micros, currency)
		VALUES (?, 0, 'CNY')
		ON CONFLICT(user_id) DO NOTHING
	`), localUserID); err != nil {
		return 0, fmt.Errorf("create imported user wallet: %w", err)
	}

	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, db.Rebind(r.sqlDialect, `
		INSERT INTO als_sub2api_auth_tokens(
			user_id,
			upstream_user_id,
			access_token,
			refresh_token,
			created_at,
			updated_at
		)
		VALUES (?, ?, '', NULL, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			upstream_user_id = excluded.upstream_user_id,
			updated_at = excluded.updated_at
	`), localUserID, upstreamUserID, now, now); err != nil {
		return 0, fmt.Errorf("bind imported upstream user id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit upstream user import: %w", err)
	}

	return localUserID, nil
}

func formatUpstreamRegisterError(status int, body []byte) string {
	message := strings.TrimSpace(extractErrorMessage(body))
	if message == "" {
		message = "sub2api register failed"
	}
	return fmt.Sprintf("%s: status %d", message, status)
}

func formatUpstreamAuthError(status int, body []byte) string {
	message := strings.TrimSpace(extractErrorMessage(body))
	if message == "" {
		message = "sub2api auth failed"
	}
	return fmt.Sprintf("%s: status %d", message, status)
}

func extractErrorMessage(body []byte) string {
	var payload map[string]any
	if len(bytes.TrimSpace(body)) == 0 || json.Unmarshal(body, &payload) != nil {
		return ""
	}
	for _, key := range []string{"error", "message", "reason"} {
		if value := strings.TrimSpace(stringFromAny(payload[key])); value != "" {
			return value
		}
	}
	if data, ok := payload["data"].(map[string]any); ok {
		for _, key := range []string{"error", "message", "reason"} {
			if value := strings.TrimSpace(stringFromAny(data[key])); value != "" {
				return value
			}
		}
	}
	return ""
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

func (r *routes) recordCheckoutSession(ctx context.Context, provider, checkoutSessionID string, userID int64, pkg adminPackageResponse, customerEmail string, amountMinor int64, currency string, feeMinor int64) error {
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
			fee_minor,
			currency,
			status,
			payload_json,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`), provider, checkoutSessionID, userID, pkg.Code, pkg.Name, strings.TrimSpace(customerEmail), amountMinor, feeMinor, strings.ToLower(strings.TrimSpace(currency)), "checkout_created", string(payloadJSON), now, now)
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
	if payload.APIKey != nil && !r.sub2api.IsConfigured() {
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
	upstreamUserID, err := r.resolveSub2APIUserID(ctx, payload.UserID)
	if err == nil {
		_, err = r.proxyClient.UpdateUserBalance(ctx, upstreamUserID, *payload.BalanceRecharge, childKey)
	}
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

	childKey := parentIdempotencyKey + ":api_key"
	_, err := r.sub2api.CreateUserAPIKeyForUser(ctx, payload.UserID, *payload.APIKey, childKey)
	_, applyErr := r.fulfillmentSvc.ApplyAPIKeyCreationResult(ctx, job.ID, err)
	if applyErr != nil {
		return applyErr
	}
	return nil
}

func (r *routes) executePackagePurchaseFulfillment(ctx context.Context, job *fulfillment.Job, payload adminPaymentSuccessRequest, parentIdempotencyKey string) error {
	pkg, err := r.loadAdminPackageByCode(ctx, strings.TrimSpace(payload.TierCode))
	if err != nil {
		_, applyErr := r.fulfillmentSvc.ApplyPackageFulfillmentResult(ctx, job.ID, err)
		if applyErr != nil {
			return applyErr
		}
		return nil
	}
	if !pkg.IsPublished {
		_, applyErr := r.fulfillmentSvc.ApplyPackageFulfillmentResult(ctx, job.ID, errors.New("package is not published"))
		if applyErr != nil {
			return applyErr
		}
		return nil
	}
	if len(pkg.GroupIDs) == 0 {
		_, applyErr := r.fulfillmentSvc.ApplyPackageFulfillmentResult(ctx, job.ID, errors.New("package has no bound groups"))
		if applyErr != nil {
			return applyErr
		}
		return nil
	}
	classifiedGroups, err := r.classifyPackageGroupIDs(ctx, pkg.GroupIDs)
	if err != nil {
		_, applyErr := r.fulfillmentSvc.ApplyPackageFulfillmentResult(ctx, job.ID, err)
		if applyErr != nil {
			return applyErr
		}
		return nil
	}
	if err := validatePackageValueForClassifiedGroups(pkg.ValueType, pkg.ValueAmount, classifiedGroups); err != nil {
		_, applyErr := r.fulfillmentSvc.ApplyPackageFulfillmentResult(ctx, job.ID, err)
		if applyErr != nil {
			return applyErr
		}
		return nil
	}

	upstreamUserID, err := r.resolveSub2APIUserID(ctx, payload.UserID)
	if err != nil {
		_, applyErr := r.fulfillmentSvc.ApplyPackageFulfillmentResult(ctx, job.ID, err)
		if applyErr != nil {
			return applyErr
		}
		return nil
	}

	var fulfillmentErr error
	isBalance := strings.TrimSpace(pkg.ValueType) == "balance"
	switch strings.TrimSpace(pkg.ValueType) {
	case "", "days", "balance":
		if strings.TrimSpace(pkg.ValueType) == "balance" {
			creditAmount := microsToFloatCurrency(pkg.ValueAmount)
			if pkg.Rate > 0 {
				// 加量包：余额 = 用户实付元 × 倍率（美元）。实付按 checkout_session_id 查 payment_records。
				paidMicros, paidErr := r.loadPaidAmountMicrosByCheckoutSession(ctx, strings.TrimSpace(payload.OrderID))
				if paidErr != nil {
					fulfillmentErr = paidErr
					break
				}
				creditAmount = microsToFloatCurrency(paidMicros) * pkg.Rate
			}
			_, fulfillmentErr = r.proxyClient.UpdateUserBalance(ctx, upstreamUserID, proxy.UpdateUserBalanceRequest{
				Balance:   creditAmount,
				Operation: "add",
				Notes:     fmt.Sprintf("stripe package purchase %s", pkg.Code),
			}, parentIdempotencyKey+":package:balance")
			if fulfillmentErr != nil {
				break
			}
		}

		if !isBalance && pkg.ValueAmount <= 0 {
			if len(classifiedGroups.SubscriptionGroupIDs) > 0 {
				fulfillmentErr = errors.New("package days value must be positive for subscription groups")
				break
			}
		}
		validityDays := int(pkg.ValueAmount)
		for _, groupID := range classifiedGroups.StandardGroupIDs {
			childKey := parentIdempotencyKey + ":grant-group:" + strconv.FormatInt(groupID, 10)
			_, fulfillmentErr = r.proxyClient.GrantUserGroup(ctx, upstreamUserID, groupID, childKey)
			if fulfillmentErr != nil {
				break
			}
			if ensureErr := r.ensurePackageUserKeyInGroup(ctx, payload.UserID, upstreamUserID, groupID, parentIdempotencyKey); ensureErr != nil {
				fulfillmentErr = ensureErr
				break
			}
		}
		if fulfillmentErr != nil {
			break
		}
		// 订阅组：days 套餐按 validityDays 授权；balance 套餐（加量包）无天数概念，跳过，避免配置冗余导致发放失败。
		if !isBalance {
			for _, groupID := range classifiedGroups.SubscriptionGroupIDs {
				fulfillmentErr = r.proxyClient.EnsureAdminSubscriptionInGroup(
					ctx,
					upstreamUserID,
					groupID,
					validityDays,
					fmt.Sprintf("stripe package purchase %s", pkg.Code),
					parentIdempotencyKey,
				)
				if fulfillmentErr != nil {
					break
				}
				if ensureErr := r.ensurePackageUserKeyInGroup(ctx, payload.UserID, upstreamUserID, groupID, parentIdempotencyKey); ensureErr != nil {
					fulfillmentErr = ensureErr
					break
				}
			}
		}
	default:
		fulfillmentErr = errors.New("package value_type is not supported for fulfillment")
	}
	if fulfillmentErr == nil {
		var subscriptionID int64
		var existed bool
		subscriptionID, _, existed, fulfillmentErr = r.ensureActiveSubscriptionForUser(ctx, payload.UserID, pkg.Code)
		if fulfillmentErr == nil {
			fulfillmentErr = r.updateLocalPackageSubscriptionExpiry(ctx, subscriptionID, pkg, existed)
		}
	}

	_, applyErr := r.fulfillmentSvc.ApplyPackageFulfillmentResult(ctx, job.ID, fulfillmentErr)
	if applyErr != nil {
		return applyErr
	}
	return nil
}

// loadPaidAmountMicrosByCheckoutSession returns the user-paid package-price portion
// (excluding surcharge) in currency micros (1e6 per yuan). Looked up by checkout_session_id
// because payment_records is created at checkout time, while fulfillment_job_id is only
// backfilled after fulfillment runs — querying by fulfillment_job_id races and may find no row.
func (r *routes) loadPaidAmountMicrosByCheckoutSession(ctx context.Context, checkoutSessionID string) (int64, error) {
	if strings.TrimSpace(checkoutSessionID) == "" {
		return 0, errors.New("missing checkout session id for paid amount lookup")
	}
	var amountMinor, feeMinor int64
	err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect,
		`SELECT amount_minor, fee_minor FROM als_payment_records WHERE checkout_session_id = ? ORDER BY id DESC LIMIT 1;`), checkoutSessionID).Scan(&amountMinor, &feeMinor)
	if err != nil {
		return 0, fmt.Errorf("load paid amount for checkout session %s: %w", checkoutSessionID, err)
	}
	// 余额基数 = 实付 − 手续费（手续费不计入充值额度）；minor(1/100) → micros(1e6) ×1e4。
	base := amountMinor - feeMinor
	if base < 0 {
		base = 0
	}
	return base * 10000, nil
}

// loadPaymentSurcharge reads admin-configurable handling-fee settings from global-vars.
// Missing/invalid → disabled (zeros).
func (r *routes) loadPaymentSurcharge(ctx context.Context) (enabled bool, feeMicros int64, thresholdMicros int64) {
	if r.configCenterSvc == nil {
		return false, 0, 0
	}
	if v, _ := r.configCenterSvc.GetGlobalVar(ctx, "payment_surcharge_enabled"); v != nil {
		enabled = strings.EqualFold(strings.TrimSpace(v.VarValue), "true")
	}
	if v, _ := r.configCenterSvc.GetGlobalVar(ctx, "payment_surcharge_amount"); v != nil {
		if yuan, err := strconv.ParseFloat(strings.TrimSpace(v.VarValue), 64); err == nil && yuan > 0 {
			feeMicros = int64(yuan * 1_000_000)
		}
	}
	if v, _ := r.configCenterSvc.GetGlobalVar(ctx, "payment_surcharge_threshold"); v != nil {
		if yuan, err := strconv.ParseFloat(strings.TrimSpace(v.VarValue), 64); err == nil && yuan > 0 {
			thresholdMicros = int64(yuan * 1_000_000)
		}
	}
	return
}

// handlePublicGetPaymentConfig exposes the surcharge rule so the marketing page can display it.
func (r *routes) handlePublicGetPaymentConfig(w http.ResponseWriter, req *http.Request) {
	enabled, feeMicros, thresholdMicros := r.loadPaymentSurcharge(req.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"surcharge_enabled":          enabled,
		"surcharge_amount_micros":    feeMicros,
		"surcharge_threshold_micros": thresholdMicros,
	})
}

func (r *routes) ensurePackageUserKeyInGroup(ctx context.Context, localUserID, upstreamUserID, groupID int64, parentIdempotencyKey string) error {
	if r.sub2api.IsConfigured() {
		hasToken, err := r.sub2api.HasUpstreamToken(ctx, localUserID)
		if err != nil {
			return err
		}
		if hasToken {
			return r.sub2api.EnsureUserKeyInGroup(ctx, localUserID, groupID, parentIdempotencyKey)
		}
	}
	if r.proxyClient == nil {
		return errors.New("proxy client is not configured")
	}
	return r.proxyClient.EnsureAdminUserKeyInGroup(ctx, upstreamUserID, groupID, parentIdempotencyKey)
}

func (r *routes) resolveSub2APIUserID(ctx context.Context, userID int64) (int64, error) {
	if userID <= 0 {
		return 0, errors.New("user id must be positive")
	}
	upstreamUserID, found, err := r.sub2api.UpstreamUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	if found {
		return upstreamUserID, nil
	}

	return userID, nil
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

func (r *routes) ensureActiveSubscriptionForUser(ctx context.Context, userID int64, tierCode string) (int64, string, bool, error) {
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
			return subscriptionID, current.TierName, true, nil
		}
	}

	subscriptionID, _, tierName, _, err := r.createOrReplaceSubscription(ctx, userID, createSubscriptionRequest{TierCode: tierCode})
	if err != nil {
		return 0, "", false, err
	}
	return subscriptionID, tierName, false, nil
}

func (r *routes) updateLocalPackageSubscriptionExpiry(ctx context.Context, subscriptionID int64, pkg adminPackageResponse, extend bool) error {
	if strings.TrimSpace(pkg.ValueType) != "days" || pkg.ValueAmount <= 0 {
		return nil
	}

	var (
		startedAtRaw string
		expiresAtRaw sql.NullString
	)
	if err := r.db.QueryRowContext(ctx, db.Rebind(r.sqlDialect, `
		SELECT started_at, expires_at
		FROM als_subscriptions
		WHERE id = ?
		LIMIT 1;
	`), subscriptionID).Scan(&startedAtRaw, &expiresAtRaw); err != nil {
		return err
	}

	now := time.Now().UTC()
	base := now
	if startedAt, ok := parseSubscriptionTime(startedAtRaw); ok {
		base = startedAt.UTC()
	}
	if extend {
		if expiresAtRaw.Valid {
			if expiresAt, ok := parseSubscriptionTime(expiresAtRaw.String); ok && expiresAt.After(base) {
				base = expiresAt.UTC()
			}
		} else if startedAt, ok := parseSubscriptionTime(startedAtRaw); ok {
			calculatedExpiry := startedAt.UTC().AddDate(0, 0, int(pkg.ValueAmount))
			if calculatedExpiry.After(base) {
				base = calculatedExpiry
			}
		}
		if now.After(base) {
			base = now
		}
	}

	newExpiresAt := base.AddDate(0, 0, int(pkg.ValueAmount)).UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, db.Rebind(r.sqlDialect, `
		UPDATE als_subscriptions
		SET expires_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?;
	`), newExpiresAt, subscriptionID)
	return err
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
			t.level,
			t.price_micros,
			t.value_type,
			t.value_amount,
			t.description,
			t.features_json,
			t.is_enabled,
			t.is_visible,
			t.is_published,
			t.created_at,
			t.updated_at,
			t.rate,
			t.min_topup_micros,
			t.max_topup_micros,
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
			level        string
			priceMicros  int64
			valueType    string
			valueAmount  int64
			description  string
			featuresJSON string
			isEnabled    bool
			isVisible    bool
			isPublished  bool
			createdAt    string
			updatedAt    string
			rate         sql.NullFloat64
			minTopup     sql.NullInt64
			maxTopup     sql.NullInt64
			groupID      sql.NullInt64
		)
		if err := rows.Scan(&tierID, &pkgCode, &pkgName, &level, &priceMicros, &valueType, &valueAmount, &description, &featuresJSON, &isEnabled, &isVisible, &isPublished, &createdAt, &updatedAt, &rate, &minTopup, &maxTopup, &groupID); err != nil {
			return nil, err
		}
		if strings.TrimSpace(level) == "" {
			level = packageLevelAdmin
		}

		idx, found := packageIndex[tierID]
		if !found {
			idx = len(packages)
			packageIndex[tierID] = idx
			packages = append(packages, adminPackageResponse{
				Code:           pkgCode,
				Name:           pkgName,
				Level:          level,
				PriceMicros:    priceMicros,
				ValueType:      valueType,
				ValueAmount:    valueAmount,
				Description:    description,
				Features:       parseFeaturesJSON(featuresJSON),
				IsEnabled:      isEnabled,
				IsVisible:      isVisible,
				IsPublished:    isPublished,
				GroupIDs:       []int64{},
				CreatedAt:      createdAt,
				UpdatedAt:      updatedAt,
				Rate:           rate.Float64,
				MinTopupMicros: minTopup.Int64,
				MaxTopupMicros: maxTopup.Int64,
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

func filterAdminPackagesByLevel(packages []adminPackageResponse, level string) []adminPackageResponse {
	filtered := make([]adminPackageResponse, 0, len(packages))
	for _, pkg := range packages {
		if pkg.Level == level {
			filtered = append(filtered, pkg)
		}
	}
	return filtered
}

func filterDistributorAssignablePackages(packages []adminPackageResponse) []adminPackageResponse {
	filtered := make([]adminPackageResponse, 0, len(packages))
	for _, pkg := range packages {
		if isDistributorAssignablePackage(pkg) {
			filtered = append(filtered, pkg)
		}
	}
	return filtered
}

func isDistributorAssignablePackage(pkg adminPackageResponse) bool {
	if !pkg.IsEnabled || !pkg.IsPublished {
		return false
	}
	return pkg.Level == packageLevelDistributor
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
	forwarded.Method = http.MethodGet
	forwarded.Body = nil
	forwarded.GetBody = nil
	forwarded.ContentLength = 0
	if err := r.sub2api.ReplaceAuthHeader(req.Context(), forwarded.Header, r); err != nil {
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
	if err := r.sub2api.ReplaceAuthHeader(req.Context(), forwarded.Header, r); err != nil {
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
			slog.Error("proxy filtered response copy failed", "path", upstreamPath, "error", copyErr)
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

func isProtectedAPIKeyPayload(payload any) (bool, error) {
	item, ok := unwrapSingleAPIKeyPayload(payload)
	if !ok {
		return false, errors.New("api key payload shape is not supported")
	}
	name := asString(item["name"])
	if name == "" {
		name = asString(item["label"])
	}
	return apikey.IsProtectedAPIKeyName(name), nil
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
	payload.Level = strings.TrimSpace(strings.ToLower(payload.Level))
	payload.ValueType = strings.TrimSpace(payload.ValueType)
	payload.Description = strings.TrimSpace(payload.Description)
	payload.FeaturesJSON = strings.TrimSpace(payload.FeaturesJSON)

	if requireCode && payload.Code == "" {
		return adminPackageRequest{}, errors.New("code is required")
	}
	if payload.Name == "" {
		return adminPackageRequest{}, errors.New("name is required")
	}
	switch payload.Level {
	case "":
		if requireCode {
			payload.Level = packageLevelAdmin
		}
	case packageLevelAdmin, packageLevelDistributor:
		// valid
	default:
		return adminPackageRequest{}, errors.New("level must be admin or distributor")
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

func packageFlagsForCreate(payload adminPackageRequest) (isVisible bool, isPublished bool) {
	isVisible = true
	isPublished = true
	if payload.IsEnabled != nil {
		isVisible = *payload.IsEnabled
		isPublished = *payload.IsEnabled
	}
	if payload.IsVisible != nil {
		isVisible = *payload.IsVisible
	}
	if payload.IsPublished != nil {
		isPublished = *payload.IsPublished
	}
	return isVisible, isPublished
}

func packageFlagsForUpdate(payload adminPackageRequest, current adminPackageResponse) (isVisible bool, isPublished bool) {
	isVisible = current.IsVisible
	isPublished = current.IsPublished
	if payload.IsEnabled != nil {
		if payload.IsVisible == nil {
			isVisible = *payload.IsEnabled
		}
		if payload.IsPublished == nil {
			isPublished = *payload.IsEnabled
		}
	}
	if payload.IsVisible != nil {
		isVisible = *payload.IsVisible
	}
	if payload.IsPublished != nil {
		isPublished = *payload.IsPublished
	}
	return isVisible, isPublished
}

func (r *routes) validatePackageGroupBindings(ctx context.Context, payload adminPackageRequest) error {
	if r.proxyClient == nil {
		return nil
	}
	classifiedGroups, err := r.classifyPackageGroupIDs(ctx, payload.GroupIDs)
	if err != nil {
		return fmt.Errorf("failed to validate package groups: %w", err)
	}
	return validatePackageValueForClassifiedGroups(payload.ValueType, payload.ValueAmount, classifiedGroups)
}

type packageGroupClassification struct {
	StandardGroupIDs     []int64
	SubscriptionGroupIDs []int64
	MissingGroupIDs      []int64
}

func (r *routes) classifyPackageGroupIDs(ctx context.Context, groupIDs []int64) (packageGroupClassification, error) {
	var classified packageGroupClassification
	if r.proxyClient == nil {
		return classified, errors.New("proxy client is not configured")
	}
	resp, err := r.proxyClient.ListAdminGroups(ctx, "")
	if err != nil {
		return classified, err
	}

	groupsByID := make(map[int64]proxy.AdminGroup, len(resp.Data))
	for _, group := range resp.Data {
		if group.ID > 0 {
			groupsByID[group.ID] = group
		}
	}

	for _, groupID := range groupIDs {
		group, ok := groupsByID[groupID]
		if !ok {
			classified.MissingGroupIDs = append(classified.MissingGroupIDs, groupID)
			continue
		}
		if isSubscriptionGroup(group) {
			classified.SubscriptionGroupIDs = append(classified.SubscriptionGroupIDs, groupID)
		} else {
			classified.StandardGroupIDs = append(classified.StandardGroupIDs, groupID)
		}
	}

	return classified, nil
}

func validatePackageValueForClassifiedGroups(valueType string, valueAmount int64, classified packageGroupClassification) error {
	if len(classified.MissingGroupIDs) > 0 {
		return fmt.Errorf("package contains groups that do not exist in sub2api: %s", joinInt64s(classified.MissingGroupIDs))
	}
	if len(classified.SubscriptionGroupIDs) > 0 {
		if strings.TrimSpace(valueType) == "balance" {
			// balance（加量包）无天数概念，绑订阅组时 fulfillment 会忽略订阅组（只加余额 + 标准组），允许。
		} else {
			if strings.TrimSpace(valueType) != "days" {
				return fmt.Errorf("packages with subscription groups must use value_type 'days' (or 'balance', which skips subscription groups); subscription group_ids: %s", joinInt64s(classified.SubscriptionGroupIDs))
			}
			if valueAmount <= 0 {
				return errors.New("value_amount must be > 0 for subscription groups")
			}
		}
	}
	return nil
}

func isSubscriptionGroup(group proxy.AdminGroup) bool {
	subscriptionType := strings.ToLower(strings.TrimSpace(group.SubscriptionType))
	if subscriptionType == "" {
		subscriptionType = strings.ToLower(strings.TrimSpace(group.Type))
	}
	switch subscriptionType {
	case "", "standard", "balance":
		return false
	case "subscription":
		return true
	default:
		return true
	}
}

func packageGroupBillingType(group proxy.AdminGroup) string {
	if isSubscriptionGroup(group) {
		return "subscription"
	}
	return "balance"
}

func joinInt64s(values []int64) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.FormatInt(value, 10))
	}
	return strings.Join(parts, ", ")
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
	if status >= 500 {
		slog.Error("HTTP error", "status", status, "message", message)
	}
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
