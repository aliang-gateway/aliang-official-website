package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"ai-api-portal/backend/internal/db"
)

func TestPublicTiersReturnsDefaultItemsWithoutAuth(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "starter", "Starter")
	itemID := insertServiceItem(t, ctx, database, "tokens_in", "Input Tokens", "token")
	insertTierDefaultItem(t, ctx, database, tierID, itemID, 1000)

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	req := httptest.NewRequest(http.MethodGet, "/public/tiers", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected content-type application/json, got %q", got)
	}

	var payload struct {
		Tiers []struct {
			Code         string `json:"code"`
			Name         string `json:"name"`
			DefaultItems []struct {
				Code          string `json:"code"`
				Name          string `json:"name"`
				Unit          string `json:"unit"`
				IncludedUnits int64  `json:"included_units"`
			} `json:"default_items"`
		} `json:"tiers"`
	}

	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(payload.Tiers) != 1 {
		t.Fatalf("expected exactly 1 tier, got %d", len(payload.Tiers))
	}

	tier := payload.Tiers[0]
	if tier.Code != "starter" || tier.Name != "Starter" {
		t.Fatalf("unexpected tier payload: %+v", tier)
	}
	if len(tier.DefaultItems) != 1 {
		t.Fatalf("expected exactly 1 default item, got %d", len(tier.DefaultItems))
	}

	item := tier.DefaultItems[0]
	if item.Code != "tokens_in" || item.Name != "Input Tokens" || item.Unit != "token" || item.IncludedUnits != 1000 {
		t.Fatalf("unexpected default item payload: %+v", item)
	}
}

func TestPublicEstimateUsesTierSpecificAndGlobalPricesAndReportsMissing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "pro", "Pro")

	inputItemID := insertServiceItem(t, ctx, database, "tokens_in", "Input Tokens", "token")
	outputItemID := insertServiceItem(t, ctx, database, "tokens_out", "Output Tokens", "token")
	vectorItemID := insertServiceItem(t, ctx, database, "vector_storage", "Vector Storage", "item")

	insertTierDefaultItem(t, ctx, database, tierID, inputItemID, 100)
	insertTierDefaultItem(t, ctx, database, tierID, outputItemID, 50)
	insertTierDefaultItem(t, ctx, database, tierID, vectorItemID, 7)

	insertUnitPrice(t, ctx, database, inputItemID, tierID, 2000, "USD", "2026-01-01T00:00:00Z")
	insertUnitPrice(t, ctx, database, outputItemID, nil, 3000, "USD", "2026-01-01T00:00:00Z")

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	body := []byte(`{"tier_code":"pro"}`)
	req := httptest.NewRequest(http.MethodPost, "/public/estimate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload struct {
		TierCode         string `json:"tier_code"`
		TierName         string `json:"tier_name"`
		Currency         string `json:"currency"`
		TotalPriceMicros int64  `json:"total_price_micros"`
		Items            []struct {
			Code               string `json:"code"`
			IncludedUnits      int64  `json:"included_units"`
			PricePerUnitMicros int64  `json:"price_per_unit_micros"`
			LineTotalMicros    int64  `json:"line_total_micros"`
			Currency           string `json:"currency"`
			MissingPrice       bool   `json:"missing_price"`
		} `json:"items"`
	}

	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.TierCode != "pro" || payload.TierName != "Pro" {
		t.Fatalf("unexpected tier estimate identity: %+v", payload)
	}
	if payload.Currency != "USD" {
		t.Fatalf("expected response currency USD, got %q", payload.Currency)
	}

	if len(payload.Items) != 3 {
		t.Fatalf("expected 3 estimate items, got %d", len(payload.Items))
	}

	itemsByCode := make(map[string]struct {
		IncludedUnits      int64
		PricePerUnitMicros int64
		LineTotalMicros    int64
		Currency           string
		MissingPrice       bool
	})
	for _, item := range payload.Items {
		itemsByCode[item.Code] = struct {
			IncludedUnits      int64
			PricePerUnitMicros int64
			LineTotalMicros    int64
			Currency           string
			MissingPrice       bool
		}{
			IncludedUnits:      item.IncludedUnits,
			PricePerUnitMicros: item.PricePerUnitMicros,
			LineTotalMicros:    item.LineTotalMicros,
			Currency:           item.Currency,
			MissingPrice:       item.MissingPrice,
		}
	}

	in := itemsByCode["tokens_in"]
	if in.MissingPrice || in.PricePerUnitMicros != 2000 || in.LineTotalMicros != 200000 || in.Currency != "USD" {
		t.Fatalf("unexpected tokens_in estimate: %+v", in)
	}

	out := itemsByCode["tokens_out"]
	if out.MissingPrice || out.PricePerUnitMicros != 3000 || out.LineTotalMicros != 150000 || out.Currency != "USD" {
		t.Fatalf("unexpected tokens_out estimate: %+v", out)
	}

	missing := itemsByCode["vector_storage"]
	if !missing.MissingPrice || missing.PricePerUnitMicros != 0 || missing.LineTotalMicros != 0 {
		t.Fatalf("expected missing price for vector_storage, got %+v", missing)
	}

	if payload.TotalPriceMicros != 350000 {
		t.Fatalf("expected total_price_micros 350000, got %d", payload.TotalPriceMicros)
	}
}

func TestAdminUnitPriceEndpointsRequireAdmin(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)
	_, nonAdminSessionToken := createUserViaAPI(t, mux, "member@example.com", "Member", "user", "")

	reqMissingAuth := httptest.NewRequest(http.MethodPut, "/admin/unit-prices", bytes.NewReader([]byte(`{"service_item_code":"tokens_in","price_per_unit_micros":123}`)))
	recMissingAuth := httptest.NewRecorder()
	mux.ServeHTTP(recMissingAuth, reqMissingAuth)
	if recMissingAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d for missing auth, got %d", http.StatusUnauthorized, recMissingAuth.Code)
	}

	reqNonAdmin := httptest.NewRequest(http.MethodPut, "/admin/unit-prices", bytes.NewReader([]byte(`{"service_item_code":"tokens_in","price_per_unit_micros":123}`)))
	setBearerAuth(reqNonAdmin, nonAdminSessionToken)
	recNonAdmin := httptest.NewRecorder()
	mux.ServeHTTP(recNonAdmin, reqNonAdmin)
	if recNonAdmin.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-admin user, got %d", http.StatusForbidden, recNonAdmin.Code)
	}
}

func TestAdminUnitPriceLifecycleSetListDeactivate(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	insertTier(t, ctx, database, "pro", "Pro")
	serviceItemID := insertServiceItem(t, ctx, database, "tokens_in", "Input Tokens", "token")

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})
	_, adminSessionToken := createUserViaAPI(t, mux, "admin@example.com", "Admin", "admin", "test-admin-secret")

	setGlobalV1Req := httptest.NewRequest(http.MethodPut, "/admin/unit-prices", bytes.NewReader([]byte(`{"service_item_code":"tokens_in","price_per_unit_micros":1000}`)))
	setBearerAuth(setGlobalV1Req, adminSessionToken)
	setGlobalV1Rec := httptest.NewRecorder()
	mux.ServeHTTP(setGlobalV1Rec, setGlobalV1Req)
	if setGlobalV1Rec.Code != http.StatusCreated {
		t.Fatalf("expected first global set status %d, got %d", http.StatusCreated, setGlobalV1Rec.Code)
	}

	setGlobalV2Req := httptest.NewRequest(http.MethodPut, "/admin/unit-prices", bytes.NewReader([]byte(`{"service_item_code":"tokens_in","price_per_unit_micros":1500,"currency":"USD"}`)))
	setBearerAuth(setGlobalV2Req, adminSessionToken)
	setGlobalV2Rec := httptest.NewRecorder()
	mux.ServeHTTP(setGlobalV2Rec, setGlobalV2Req)
	if setGlobalV2Rec.Code != http.StatusCreated {
		t.Fatalf("expected second global set status %d, got %d", http.StatusCreated, setGlobalV2Rec.Code)
	}

	setTierReq := httptest.NewRequest(http.MethodPut, "/admin/unit-prices", bytes.NewReader([]byte(`{"service_item_code":"tokens_in","tier_code":"pro","price_per_unit_micros":2500,"currency":"EUR"}`)))
	setBearerAuth(setTierReq, adminSessionToken)
	setTierRec := httptest.NewRecorder()
	mux.ServeHTTP(setTierRec, setTierReq)
	if setTierRec.Code != http.StatusCreated {
		t.Fatalf("expected tier-specific set status %d, got %d", http.StatusCreated, setTierRec.Code)
	}

	var (
		oldGlobalEffectiveTo sql.NullString
		activeGlobalCount    int64
	)
	err := database.QueryRowContext(ctx, `
		SELECT effective_to
		FROM unit_prices
		WHERE service_item_id = ? AND tier_id IS NULL
		ORDER BY effective_from ASC, id ASC
		LIMIT 1;
	`, serviceItemID).Scan(&oldGlobalEffectiveTo)
	if err != nil {
		t.Fatalf("query first global unit price: %v", err)
	}
	if !oldGlobalEffectiveTo.Valid {
		t.Fatalf("expected first global unit price to be ended")
	}

	err = database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM unit_prices
		WHERE service_item_id = ? AND tier_id IS NULL AND effective_to IS NULL;
	`, serviceItemID).Scan(&activeGlobalCount)
	if err != nil {
		t.Fatalf("count active global unit prices: %v", err)
	}
	if activeGlobalCount != 1 {
		t.Fatalf("expected exactly one active global unit price, got %d", activeGlobalCount)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/admin/unit-prices?service_item_code=tokens_in", nil)
	setBearerAuth(listReq, adminSessionToken)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d", http.StatusOK, listRec.Code)
	}

	var listPayload struct {
		UnitPrices []struct {
			ServiceItemCode    string  `json:"service_item_code"`
			TierCode           *string `json:"tier_code"`
			PricePerUnitMicros int64   `json:"price_per_unit_micros"`
			Currency           string  `json:"currency"`
			EffectiveFrom      string  `json:"effective_from"`
		} `json:"unit_prices"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listPayload.UnitPrices) != 2 {
		t.Fatalf("expected 2 active unit prices (global + tier), got %d", len(listPayload.UnitPrices))
	}

	var (
		sawGlobal bool
		sawTier   bool
	)
	for _, p := range listPayload.UnitPrices {
		if p.ServiceItemCode != "tokens_in" {
			t.Fatalf("unexpected service_item_code %q", p.ServiceItemCode)
		}
		if p.TierCode == nil {
			sawGlobal = true
			if p.PricePerUnitMicros != 1500 || p.Currency != "USD" {
				t.Fatalf("unexpected active global unit price payload: %+v", p)
			}
			continue
		}
		if *p.TierCode == "pro" {
			sawTier = true
			if p.PricePerUnitMicros != 2500 || p.Currency != "EUR" {
				t.Fatalf("unexpected active tier unit price payload: %+v", p)
			}
		}
	}
	if !sawGlobal || !sawTier {
		t.Fatalf("expected list payload to include both active global and tier-specific prices")
	}

	listTierReq := httptest.NewRequest(http.MethodGet, "/admin/unit-prices?service_item_code=tokens_in&tier_code=pro", nil)
	setBearerAuth(listTierReq, adminSessionToken)
	listTierRec := httptest.NewRecorder()
	mux.ServeHTTP(listTierRec, listTierReq)
	if listTierRec.Code != http.StatusOK {
		t.Fatalf("expected tier-filtered list status %d, got %d", http.StatusOK, listTierRec.Code)
	}

	if err := json.NewDecoder(listTierRec.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode tier-filtered list response: %v", err)
	}
	if len(listPayload.UnitPrices) != 1 {
		t.Fatalf("expected 1 tier-filtered active unit price, got %d", len(listPayload.UnitPrices))
	}
	if listPayload.UnitPrices[0].TierCode == nil || *listPayload.UnitPrices[0].TierCode != "pro" {
		t.Fatalf("expected tier-filtered response to return pro tier price, got %+v", listPayload.UnitPrices[0])
	}

	deactivateGlobalReq := httptest.NewRequest(http.MethodDelete, "/admin/unit-prices?service_item_code=tokens_in", nil)
	setBearerAuth(deactivateGlobalReq, adminSessionToken)
	deactivateGlobalRec := httptest.NewRecorder()
	mux.ServeHTTP(deactivateGlobalRec, deactivateGlobalReq)
	if deactivateGlobalRec.Code != http.StatusOK {
		t.Fatalf("expected deactivate global status %d, got %d", http.StatusOK, deactivateGlobalRec.Code)
	}

	err = database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM unit_prices
		WHERE service_item_id = ? AND tier_id IS NULL AND effective_to IS NULL;
	`, serviceItemID).Scan(&activeGlobalCount)
	if err != nil {
		t.Fatalf("count active globals after deactivation: %v", err)
	}
	if activeGlobalCount != 0 {
		t.Fatalf("expected 0 active global prices after deactivation, got %d", activeGlobalCount)
	}

	deactivateTierReq := httptest.NewRequest(http.MethodDelete, "/admin/unit-prices?service_item_code=tokens_in&tier_code=pro", nil)
	setBearerAuth(deactivateTierReq, adminSessionToken)
	deactivateTierRec := httptest.NewRecorder()
	mux.ServeHTTP(deactivateTierRec, deactivateTierReq)
	if deactivateTierRec.Code != http.StatusOK {
		t.Fatalf("expected deactivate tier status %d, got %d", http.StatusOK, deactivateTierRec.Code)
	}

	listAfterDeactivateReq := httptest.NewRequest(http.MethodGet, "/admin/unit-prices?service_item_code=tokens_in", nil)
	setBearerAuth(listAfterDeactivateReq, adminSessionToken)
	listAfterDeactivateRec := httptest.NewRecorder()
	mux.ServeHTTP(listAfterDeactivateRec, listAfterDeactivateReq)
	if listAfterDeactivateRec.Code != http.StatusOK {
		t.Fatalf("expected post-deactivate list status %d, got %d", http.StatusOK, listAfterDeactivateRec.Code)
	}

	if err := json.NewDecoder(listAfterDeactivateRec.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode post-deactivate list response: %v", err)
	}
	if len(listPayload.UnitPrices) != 0 {
		t.Fatalf("expected no active prices after deactivating both scopes, got %d", len(listPayload.UnitPrices))
	}
}

func TestAdminSetUnitPriceValidation(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	insertTier(t, ctx, database, "pro", "Pro")
	insertServiceItem(t, ctx, database, "tokens_in", "Input Tokens", "token")

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})
	_, adminSessionToken := createUserViaAPI(t, mux, "admin2@example.com", "Admin Two", "admin", "test-admin-secret")

	testCases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "missing service item code", body: `{"price_per_unit_micros":1000}`, wantStatus: http.StatusBadRequest},
		{name: "unknown service item code", body: `{"service_item_code":"missing","price_per_unit_micros":1000}`, wantStatus: http.StatusBadRequest},
		{name: "unknown tier code", body: `{"service_item_code":"tokens_in","tier_code":"missing","price_per_unit_micros":1000}`, wantStatus: http.StatusBadRequest},
		{name: "negative price", body: `{"service_item_code":"tokens_in","price_per_unit_micros":-1}`, wantStatus: http.StatusBadRequest},
		{name: "invalid currency lowercase", body: `{"service_item_code":"tokens_in","price_per_unit_micros":1000,"currency":"usd"}`, wantStatus: http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/admin/unit-prices", bytes.NewReader([]byte(tc.body)))
			setBearerAuth(req, adminSessionToken)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, rec.Code)
			}
		})
	}
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()
	dbFile := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(ctx, dbFile)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := db.ApplyMigrations(ctx, database); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}

	return database
}

func insertTier(t *testing.T, ctx context.Context, database *sql.DB, code, name string) int64 {
	t.Helper()

	result, err := database.ExecContext(ctx, `INSERT INTO tiers(code, name) VALUES (?, ?);`, code, name)
	if err != nil {
		t.Fatalf("insert tier error = %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("insert tier LastInsertId error = %v", err)
	}

	return id
}

func insertServiceItem(t *testing.T, ctx context.Context, database *sql.DB, code, name, unit string) int64 {
	t.Helper()

	result, err := database.ExecContext(ctx, `INSERT INTO service_items(code, name, unit) VALUES (?, ?, ?);`, code, name, unit)
	if err != nil {
		t.Fatalf("insert service item error = %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("insert service item LastInsertId error = %v", err)
	}

	return id
}

func insertTierDefaultItem(t *testing.T, ctx context.Context, database *sql.DB, tierID, serviceItemID, includedUnits int64) {
	t.Helper()

	_, err := database.ExecContext(ctx, `
		INSERT INTO tier_default_items(tier_id, service_item_id, included_units)
		VALUES (?, ?, ?);
	`, tierID, serviceItemID, includedUnits)
	if err != nil {
		t.Fatalf("insert tier default item error = %v", err)
	}
}

func insertUnitPrice(t *testing.T, ctx context.Context, database *sql.DB, serviceItemID int64, tierID any, pricePerUnitMicros int64, currency, effectiveFrom string) {
	t.Helper()

	_, err := database.ExecContext(ctx, `
		INSERT INTO unit_prices(service_item_id, tier_id, price_per_unit_micros, currency, effective_from)
		VALUES (?, ?, ?, ?, ?);
	`, serviceItemID, tierID, pricePerUnitMicros, currency, effectiveFrom)
	if err != nil {
		t.Fatalf("insert unit price error = %v", err)
	}
}
