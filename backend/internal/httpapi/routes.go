package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"ai-api-portal/backend/internal/apikey"
	"ai-api-portal/backend/internal/auth"
)

type routes struct {
	db     *sql.DB
	apiKey *apikey.Service
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
	r := &routes{db: database, apiKey: apikey.NewService(database)}
	authenticated := auth.RequireUser(database)

	mux.HandleFunc("POST /users", r.handleCreateUser)
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
		expectedSecret := strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_SECRET"))
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
