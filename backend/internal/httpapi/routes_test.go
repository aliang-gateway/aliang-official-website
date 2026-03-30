package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/fulfillment"
	"ai-api-portal/backend/internal/proxy"
	portalstripe "ai-api-portal/backend/internal/stripe"
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
		FROM als_unit_prices
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
		FROM als_unit_prices
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
		FROM als_unit_prices
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

func TestAdminPaymentSuccessEndpointRequiresAdmin(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})
	_, nonAdminSessionToken := createUserViaAPI(t, mux, "member-payment@example.com", "Member Payment", "user", "")

	body := `{"payment_event_id":"evt_1","user_id":1}`
	reqMissingAuth := httptest.NewRequest(http.MethodPost, "/admin/fulfillment/payment-success", bytes.NewReader([]byte(body)))
	reqMissingAuth.Header.Set("Idempotency-Key", "idem-payment-1")
	recMissingAuth := httptest.NewRecorder()
	mux.ServeHTTP(recMissingAuth, reqMissingAuth)
	if recMissingAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d for missing auth, got %d", http.StatusUnauthorized, recMissingAuth.Code)
	}

	reqNonAdmin := httptest.NewRequest(http.MethodPost, "/admin/fulfillment/payment-success", bytes.NewReader([]byte(body)))
	reqNonAdmin.Header.Set("Idempotency-Key", "idem-payment-2")
	setBearerAuth(reqNonAdmin, nonAdminSessionToken)
	recNonAdmin := httptest.NewRecorder()
	mux.ServeHTTP(recNonAdmin, reqNonAdmin)
	if recNonAdmin.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-admin user, got %d", http.StatusForbidden, recNonAdmin.Code)
	}
}

func TestAdminPaymentSuccessIngestionLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})
	userID, _ := createUserViaAPI(t, mux, "target-payment@example.com", "Target Payment", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "admin-payment@example.com", "Admin Payment", "admin", "test-admin-secret")

	body := `{"payment_event_id":"evt_success_1","order_id":"ord_1","provider":"stripe","user_id":` + strconv.FormatInt(userID, 10) + `,"payload":{"invoice_id":"in_1"}}`
	firstReq := httptest.NewRequest(http.MethodPost, "/admin/fulfillment/payment-success", bytes.NewReader([]byte(body)))
	firstReq.Header.Set("Idempotency-Key", "idem-payment-ok")
	setBearerAuth(firstReq, adminSessionToken)
	firstRec := httptest.NewRecorder()
	mux.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusAccepted {
		t.Fatalf("expected first status %d, got %d body=%s", http.StatusAccepted, firstRec.Code, firstRec.Body.String())
	}

	var firstPayload struct {
		Job struct {
			ID             int64   `json:"id"`
			EventType      string  `json:"event_type"`
			Status         string  `json:"status"`
			UserID         *int64  `json:"user_id"`
			SubscriptionID *int64  `json:"subscription_id"`
			IdempotencyKey *string `json:"idempotency_key"`
		} `json:"job"`
	}
	if err := json.NewDecoder(firstRec.Body).Decode(&firstPayload); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if firstPayload.Job.ID <= 0 || firstPayload.Job.Status != "paid_unfulfilled" || firstPayload.Job.EventType != "payment_succeeded" {
		t.Fatalf("unexpected first job payload: %+v", firstPayload.Job)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/admin/fulfillment/payment-success", bytes.NewReader([]byte(body)))
	secondReq.Header.Set("Idempotency-Key", "idem-payment-ok")
	setBearerAuth(secondReq, adminSessionToken)
	secondRec := httptest.NewRecorder()
	mux.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusAccepted {
		t.Fatalf("expected replay status %d, got %d body=%s", http.StatusAccepted, secondRec.Code, secondRec.Body.String())
	}

	var secondPayload struct {
		Job struct {
			ID int64 `json:"id"`
		} `json:"job"`
	}
	if err := json.NewDecoder(secondRec.Body).Decode(&secondPayload); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if secondPayload.Job.ID != firstPayload.Job.ID {
		t.Fatalf("expected replay to return same job id, got first=%d second=%d", firstPayload.Job.ID, secondPayload.Job.ID)
	}

	var jobCount int64
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_fulfillment_jobs WHERE idempotency_key = ?;`, "idem-payment-ok").Scan(&jobCount); err != nil {
		t.Fatalf("count fulfillment jobs: %v", err)
	}
	if jobCount != 1 {
		t.Fatalf("expected one fulfillment job, got %d", jobCount)
	}

	var eventCount int64
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_fulfillment_events WHERE fulfillment_job_id = ?;`, firstPayload.Job.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count fulfillment events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected one fulfillment event for replay-safe ingest, got %d", eventCount)
	}
}

func TestAdminPaymentSuccessRejectsMissingIdempotencyKeyAndConflicts(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})
	userID, _ := createUserViaAPI(t, mux, "target-conflict@example.com", "Target Conflict", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "admin-conflict@example.com", "Admin Conflict", "admin", "test-admin-secret")

	body := `{"payment_event_id":"evt_conflict_1","user_id":` + strconv.FormatInt(userID, 10) + `}`
	missingKeyReq := httptest.NewRequest(http.MethodPost, "/admin/fulfillment/payment-success", bytes.NewReader([]byte(body)))
	setBearerAuth(missingKeyReq, adminSessionToken)
	missingKeyRec := httptest.NewRecorder()
	mux.ServeHTTP(missingKeyRec, missingKeyReq)
	if missingKeyRec.Code != http.StatusBadRequest {
		t.Fatalf("expected missing key status %d, got %d body=%s", http.StatusBadRequest, missingKeyRec.Code, missingKeyRec.Body.String())
	}
	if !strings.Contains(missingKeyRec.Body.String(), "idempotency key is required") {
		t.Fatalf("expected missing key error body, got %s", missingKeyRec.Body.String())
	}

	firstReq := httptest.NewRequest(http.MethodPost, "/admin/fulfillment/payment-success", bytes.NewReader([]byte(body)))
	firstReq.Header.Set("Idempotency-Key", "idem-payment-conflict")
	setBearerAuth(firstReq, adminSessionToken)
	firstRec := httptest.NewRecorder()
	mux.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusAccepted {
		t.Fatalf("expected first status %d, got %d body=%s", http.StatusAccepted, firstRec.Code, firstRec.Body.String())
	}

	conflictBody := `{"payment_event_id":"evt_conflict_2","user_id":` + strconv.FormatInt(userID, 10) + `}`
	conflictReq := httptest.NewRequest(http.MethodPost, "/admin/fulfillment/payment-success", bytes.NewReader([]byte(conflictBody)))
	conflictReq.Header.Set("Idempotency-Key", "idem-payment-conflict")
	setBearerAuth(conflictReq, adminSessionToken)
	conflictRec := httptest.NewRecorder()
	mux.ServeHTTP(conflictRec, conflictReq)
	if conflictRec.Code != http.StatusConflict {
		t.Fatalf("expected conflict status %d, got %d body=%s", http.StatusConflict, conflictRec.Code, conflictRec.Body.String())
	}
}

func TestCreatePackageCheckoutSessionReturnsStripeURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "pro-monthly", "Pro Monthly")
	if _, err := database.ExecContext(ctx, `UPDATE als_tiers SET price_micros = 29900000, value_type = 'days', value_amount = 30, is_enabled = 1 WHERE id = ?;`, tierID); err != nil {
		t.Fatalf("update tier fields: %v", err)
	}
	insertTierGroupBinding(t, ctx, database, tierID, 77)

	stripeClient, err := portalstripe.NewClientWithHTTPClient(portalstripe.Config{
		SecretKey:     "sk_test_checkout",
		WebhookSecret: "whsec_checkout",
		SuccessURL:    "https://portal.example.com/dashboard?checkout=success",
		CancelURL:     "https://portal.example.com/dashboard?checkout=cancelled",
		Currency:      "cny",
	}, &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://api.stripe.com/v1/checkout/sessions" {
				t.Fatalf("unexpected stripe url: %s", req.URL.String())
			}
			if req.Method != http.MethodPost {
				t.Fatalf("unexpected stripe method: %s", req.Method)
			}
			if got := req.Header.Get("Authorization"); got != "Bearer sk_test_checkout" {
				t.Fatalf("unexpected stripe authorization: %q", got)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read stripe request body: %v", err)
			}
			encoded := string(body)
			if !strings.Contains(encoded, "metadata%5Btier_code%5D=pro-monthly") {
				t.Fatalf("expected tier_code metadata in stripe request body, got %s", encoded)
			}
			if !strings.Contains(encoded, "line_items%5B0%5D%5Bprice_data%5D%5Bunit_amount%5D=2990") {
				t.Fatalf("expected cny unit_amount in stripe request body, got %s", encoded)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"id":"cs_test_123","url":"https://checkout.stripe.com/c/pay/cs_test_123"}`)),
			}, nil
		}),
	})
	if err != nil {
		t.Fatalf("new stripe client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{
		AdminBootstrapSecret: "test-admin-secret",
		StripeClient:         stripeClient,
	})
	userID, sessionToken := createUserViaAPI(t, mux, "checkout-user@example.com", "Checkout User", "user", "")

	req := httptest.NewRequest(http.MethodPost, "/checkout/package", bytes.NewBufferString(`{"tier_code":"pro-monthly"}`))
	req.Header.Set("Content-Type", "application/json")
	setBearerAuth(req, sessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	var payload struct {
		SessionID   string `json:"session_id"`
		CheckoutURL string `json:"checkout_url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode checkout response: %v", err)
	}
	if payload.SessionID != "cs_test_123" || payload.CheckoutURL == "" {
		t.Fatalf("unexpected checkout response: %+v", payload)
	}
	if userID <= 0 {
		t.Fatalf("expected positive user id, got %d", userID)
	}

	var (
		recordStatus string
		recordTier   string
	)
	if err := database.QueryRowContext(ctx, `SELECT status, tier_code FROM als_payment_records WHERE checkout_session_id = ?;`, "cs_test_123").Scan(&recordStatus, &recordTier); err != nil {
		t.Fatalf("query payment record: %v", err)
	}
	if recordStatus != "checkout_created" || recordTier != "pro-monthly" {
		t.Fatalf("unexpected payment record values: status=%q tier=%q", recordStatus, recordTier)
	}
}

func TestStripeWebhookCreatesLocalSubscriptionAndRedeemsPackage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "pro-monthly", "Pro Monthly")
	if _, err := database.ExecContext(ctx, `UPDATE als_tiers SET price_micros = 29900000, value_type = 'days', value_amount = 30, is_enabled = 1 WHERE id = ?;`, tierID); err != nil {
		t.Fatalf("update tier fields: %v", err)
	}
	insertTierGroupBinding(t, ctx, database, tierID, 77)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/admin/redeem-codes/create-and-redeem" {
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if got := req.Header.Get("Idempotency-Key"); got != "stripe:evt_stripe_1:package:77" {
			t.Fatalf("unexpected idempotency key: %q", got)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read upstream body: %v", err)
		}
		if !strings.Contains(string(body), `"type":"subscription"`) || !strings.Contains(string(body), `"group_id":77`) || !strings.Contains(string(body), `"validity_days":30`) {
			t.Fatalf("unexpected create-and-redeem payload: %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"redeem_code":{"id":1,"code":"ORDER-G77","type":"subscription","value":0,"status":"used","group_id":77,"validity_days":30}}}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}
	stripeClient, err := portalstripe.NewClient(portalstripe.Config{
		SecretKey:     "sk_test_webhook",
		WebhookSecret: "whsec_test_webhook",
		SuccessURL:    "https://portal.example.com/dashboard?checkout=success",
		CancelURL:     "https://portal.example.com/dashboard?checkout=cancelled",
		Currency:      "cny",
	})
	if err != nil {
		t.Fatalf("new stripe client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{
		AdminBootstrapSecret: "test-admin-secret",
		ProxyClient:          proxyClient,
		StripeClient:         stripeClient,
	})
	userID, _ := createUserViaAPI(t, mux, "stripe-webhook-user@example.com", "Stripe Webhook User", "user", "")

	eventBody := []byte(fmt.Sprintf(`{
	  "id":"evt_stripe_1",
	  "type":"checkout.session.completed",
	  "data":{
	    "object":{
	      "id":"cs_test_webhook",
	      "payment_status":"paid",
	      "metadata":{"user_id":"%d","tier_code":"pro-monthly"},
	      "amount_total":2990,
	      "currency":"cny",
	      "customer_email":"stripe-webhook-user@example.com"
	    }
	  }
	}`, userID))
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader(eventBody))
	req.Header.Set("Stripe-Signature", stripeTestSignature("whsec_test_webhook", time.Now().UTC().Unix(), eventBody))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var subCount int64
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_subscriptions WHERE user_id = ? AND status = 'active' AND ended_at IS NULL;`, userID).Scan(&subCount); err != nil {
		t.Fatalf("count active als_subscriptions: %v", err)
	}
	if subCount != 1 {
		t.Fatalf("expected one active local subscription, got %d", subCount)
	}

	var finalStatus string
	if err := database.QueryRowContext(ctx, `SELECT status FROM als_fulfillment_jobs WHERE idempotency_key = ?;`, "stripe:evt_stripe_1").Scan(&finalStatus); err != nil {
		t.Fatalf("query fulfillment job status: %v", err)
	}
	if finalStatus != "fulfilled" {
		t.Fatalf("expected fulfilled job, got %q", finalStatus)
	}

	var (
		recordStatus      string
		recordEventID     string
		recordTierCode    string
		recordFulfillID   sql.NullInt64
	)
	if err := database.QueryRowContext(ctx, `SELECT status, payment_event_id, tier_code, fulfillment_job_id FROM als_payment_records WHERE checkout_session_id = ?;`, "cs_test_webhook").Scan(&recordStatus, &recordEventID, &recordTierCode, &recordFulfillID); err != nil {
		t.Fatalf("query payment record after webhook: %v", err)
	}
	if recordStatus != "payment_succeeded" || recordEventID != "evt_stripe_1" || recordTierCode != "pro-monthly" {
		t.Fatalf("unexpected payment record after webhook: status=%q event=%q tier=%q", recordStatus, recordEventID, recordTierCode)
	}
	if !recordFulfillID.Valid || recordFulfillID.Int64 <= 0 {
		t.Fatalf("expected fulfillment_job_id to be recorded, got %+v", recordFulfillID)
	}
}

func TestAdminGetFulfillmentJobRequiresAdminAndReturnsJob(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})
	userID, _ := createUserViaAPI(t, mux, "fulfillment-target@example.com", "Fulfillment Target", "user", "")
	_, nonAdminSessionToken := createUserViaAPI(t, mux, "fulfillment-member@example.com", "Fulfillment Member", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "fulfillment-admin@example.com", "Fulfillment Admin", "admin", "test-admin-secret")

	body := `{"payment_event_id":"evt_admin_get_1","user_id":` + strconv.FormatInt(userID, 10) + `}`
	createReq := httptest.NewRequest(http.MethodPost, "/admin/fulfillment/payment-success", bytes.NewReader([]byte(body)))
	createReq.Header.Set("Idempotency-Key", "idem-admin-get-1")
	setBearerAuth(createReq, adminSessionToken)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusAccepted {
		t.Fatalf("expected create status %d, got %d body=%s", http.StatusAccepted, createRec.Code, createRec.Body.String())
	}

	var createPayload struct {
		Job struct {
			ID int64 `json:"id"`
		} `json:"job"`
	}
	if err := json.NewDecoder(createRec.Body).Decode(&createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	missingAuthReq := httptest.NewRequest(http.MethodGet, "/admin/fulfillment/jobs/"+strconv.FormatInt(createPayload.Job.ID, 10), nil)
	missingAuthRec := httptest.NewRecorder()
	mux.ServeHTTP(missingAuthRec, missingAuthReq)
	if missingAuthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected missing-auth status %d, got %d", http.StatusUnauthorized, missingAuthRec.Code)
	}

	nonAdminReq := httptest.NewRequest(http.MethodGet, "/admin/fulfillment/jobs/"+strconv.FormatInt(createPayload.Job.ID, 10), nil)
	setBearerAuth(nonAdminReq, nonAdminSessionToken)
	nonAdminRec := httptest.NewRecorder()
	mux.ServeHTTP(nonAdminRec, nonAdminReq)
	if nonAdminRec.Code != http.StatusForbidden {
		t.Fatalf("expected non-admin status %d, got %d", http.StatusForbidden, nonAdminRec.Code)
	}

	adminReq := httptest.NewRequest(http.MethodGet, "/admin/fulfillment/jobs/"+strconv.FormatInt(createPayload.Job.ID, 10), nil)
	setBearerAuth(adminReq, adminSessionToken)
	adminRec := httptest.NewRecorder()
	mux.ServeHTTP(adminRec, adminReq)
	if adminRec.Code != http.StatusOK {
		t.Fatalf("expected admin get status %d, got %d body=%s", http.StatusOK, adminRec.Code, adminRec.Body.String())
	}

	var payload struct {
		Job struct {
			ID           int64   `json:"id"`
			Status       string  `json:"status"`
			EventType    string  `json:"event_type"`
			UserID       *int64  `json:"user_id"`
			ErrorMessage *string `json:"error_message"`
		} `json:"job"`
	}
	if err := json.NewDecoder(adminRec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode admin get response: %v", err)
	}
	if payload.Job.ID != createPayload.Job.ID || payload.Job.Status != "paid_unfulfilled" || payload.Job.EventType != "payment_succeeded" {
		t.Fatalf("unexpected admin fulfillment payload: %+v", payload.Job)
	}
	if payload.Job.UserID == nil || *payload.Job.UserID != userID {
		t.Fatalf("expected admin payload user_id=%d, got %+v", userID, payload.Job.UserID)
	}
	if payload.Job.ErrorMessage != nil {
		t.Fatalf("expected nil error_message for fresh job, got %q", *payload.Job.ErrorMessage)
	}

	notFoundReq := httptest.NewRequest(http.MethodGet, "/admin/fulfillment/jobs/999999", nil)
	setBearerAuth(notFoundReq, adminSessionToken)
	notFoundRec := httptest.NewRecorder()
	mux.ServeHTTP(notFoundRec, notFoundReq)
	if notFoundRec.Code != http.StatusNotFound {
		t.Fatalf("expected not found status %d, got %d", http.StatusNotFound, notFoundRec.Code)
	}
}

func TestUserGetFulfillmentJobOwnsAccess(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})
	ownerUserID, ownerSessionToken := createUserViaAPI(t, mux, "fulfillment-owner@example.com", "Fulfillment Owner", "user", "")
	otherUserID, otherSessionToken := createUserViaAPI(t, mux, "fulfillment-other@example.com", "Fulfillment Other", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "fulfillment-admin2@example.com", "Fulfillment Admin Two", "admin", "test-admin-secret")

	body := `{"payment_event_id":"evt_user_get_1","user_id":` + strconv.FormatInt(ownerUserID, 10) + `}`
	createReq := httptest.NewRequest(http.MethodPost, "/admin/fulfillment/payment-success", bytes.NewReader([]byte(body)))
	createReq.Header.Set("Idempotency-Key", "idem-user-get-1")
	setBearerAuth(createReq, adminSessionToken)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusAccepted {
		t.Fatalf("expected create status %d, got %d body=%s", http.StatusAccepted, createRec.Code, createRec.Body.String())
	}

	var createPayload struct {
		Job struct {
			ID int64 `json:"id"`
		} `json:"job"`
	}
	if err := json.NewDecoder(createRec.Body).Decode(&createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	ownerReq := httptest.NewRequest(http.MethodGet, "/fulfillment/jobs/"+strconv.FormatInt(createPayload.Job.ID, 10), nil)
	setBearerAuth(ownerReq, ownerSessionToken)
	ownerRec := httptest.NewRecorder()
	mux.ServeHTTP(ownerRec, ownerReq)
	if ownerRec.Code != http.StatusOK {
		t.Fatalf("expected owner get status %d, got %d body=%s", http.StatusOK, ownerRec.Code, ownerRec.Body.String())
	}

	var ownerPayload struct {
		Job struct {
			ID        int64  `json:"id"`
			Status    string `json:"status"`
			EventType string `json:"event_type"`
			UserID    *int64 `json:"user_id"`
		} `json:"job"`
	}
	if err := json.NewDecoder(ownerRec.Body).Decode(&ownerPayload); err != nil {
		t.Fatalf("decode owner response: %v", err)
	}
	if ownerPayload.Job.ID != createPayload.Job.ID || ownerPayload.Job.Status != "paid_unfulfilled" || ownerPayload.Job.EventType != "payment_succeeded" {
		t.Fatalf("unexpected owner fulfillment payload: %+v", ownerPayload.Job)
	}
	if ownerPayload.Job.UserID == nil || *ownerPayload.Job.UserID != ownerUserID {
		t.Fatalf("expected owner payload user_id=%d, got %+v", ownerUserID, ownerPayload.Job.UserID)
	}

	otherReq := httptest.NewRequest(http.MethodGet, "/fulfillment/jobs/"+strconv.FormatInt(createPayload.Job.ID, 10), nil)
	setBearerAuth(otherReq, otherSessionToken)
	otherRec := httptest.NewRecorder()
	mux.ServeHTTP(otherRec, otherReq)
	if otherRec.Code != http.StatusForbidden {
		t.Fatalf("expected other-user status %d, got %d body=%s", http.StatusForbidden, otherRec.Code, otherRec.Body.String())
	}

	missingAuthReq := httptest.NewRequest(http.MethodGet, "/fulfillment/jobs/"+strconv.FormatInt(createPayload.Job.ID, 10), nil)
	missingAuthRec := httptest.NewRecorder()
	mux.ServeHTTP(missingAuthRec, missingAuthReq)
	if missingAuthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected missing-auth status %d, got %d", http.StatusUnauthorized, missingAuthRec.Code)
	}

	invalidIDReq := httptest.NewRequest(http.MethodGet, "/fulfillment/jobs/not-a-number", nil)
	setBearerAuth(invalidIDReq, ownerSessionToken)
	invalidIDRec := httptest.NewRecorder()
	mux.ServeHTTP(invalidIDRec, invalidIDReq)
	if invalidIDRec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid-id status %d, got %d", http.StatusBadRequest, invalidIDRec.Code)
	}

	notFoundReq := httptest.NewRequest(http.MethodGet, "/fulfillment/jobs/999999", nil)
	setBearerAuth(notFoundReq, ownerSessionToken)
	notFoundRec := httptest.NewRecorder()
	mux.ServeHTTP(notFoundRec, notFoundReq)
	if notFoundRec.Code != http.StatusNotFound {
		t.Fatalf("expected not found status %d, got %d", http.StatusNotFound, notFoundRec.Code)
	}

	if otherUserID == ownerUserID {
		t.Fatalf("expected distinct als_users for ownership test")
	}
}

func TestAdminPaymentSuccessExecutesBalanceRechargeImmediately(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/admin/users/1/balance" {
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if got := req.Header.Get("Idempotency-Key"); got != "idem-balance-run:balance" {
			t.Fatalf("expected child idempotency key, got %q", got)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read upstream body: %v", err)
		}
		if !strings.Contains(string(body), `"balance":25`) || !strings.Contains(string(body), `"operation":"add"`) {
			t.Fatalf("unexpected balance payload: %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":1,"balance":125}}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	userID, _ := createUserViaAPI(t, mux, "balance-target@example.com", "Balance Target", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "balance-admin@example.com", "Balance Admin", "admin", "test-admin-secret")

	if userID != 1 {
		t.Fatalf("expected first created user id to be 1 for upstream route assertion, got %d", userID)
	}

	body := `{"payment_event_id":"evt_balance_1","user_id":` + strconv.FormatInt(userID, 10) + `,"balance_recharge":{"balance":25,"operation":"add","notes":"package purchase"}}`
	req := httptest.NewRequest(http.MethodPost, "/admin/fulfillment/payment-success", bytes.NewReader([]byte(body)))
	req.Header.Set("Idempotency-Key", "idem-balance-run")
	setBearerAuth(req, adminSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	var payload struct {
		Job struct {
			ID           int64   `json:"id"`
			Status       string  `json:"status"`
			EventType    string  `json:"event_type"`
			ErrorMessage *string `json:"error_message"`
		} `json:"job"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Job.Status != "fulfilled" {
		t.Fatalf("expected fulfilled job after balance execution, got %+v", payload.Job)
	}

	var eventCount int64
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_fulfillment_events WHERE fulfillment_job_id = ?;`, payload.Job.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count fulfillment events: %v", err)
	}
	if eventCount < 2 {
		t.Fatalf("expected at least two fulfillment events after synchronous execution, got %d", eventCount)
	}

	var finalStatus string
	if err := database.QueryRowContext(ctx, `SELECT status FROM als_fulfillment_jobs WHERE id = ?;`, payload.Job.ID).Scan(&finalStatus); err != nil {
		t.Fatalf("query final job status: %v", err)
	}
	if finalStatus != "fulfilled" {
		t.Fatalf("expected persisted status fulfilled, got %q", finalStatus)
	}
}

func TestAdminPaymentSuccessExecutesDelegatedAPIKeyCreationImmediately(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/keys" {
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer upstream-user-token" {
			t.Fatalf("expected delegated bearer token, got %q", got)
		}
		if got := req.Header.Get("Idempotency-Key"); got != "idem-api-key-run:api_key" {
			t.Fatalf("expected child api key idempotency key, got %q", got)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read upstream body: %v", err)
		}
		if !strings.Contains(string(body), `"name":"starter-key"`) || !strings.Contains(string(body), `"group_id":77`) {
			t.Fatalf("unexpected api key payload: %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":91,"name":"starter-key","key":"sk-live","group_id":77,"user_id":1,"status":"active"}}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	userID, _ := createUserViaAPI(t, mux, "apikey-target@example.com", "API Key Target", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "apikey-admin@example.com", "API Key Admin", "admin", "test-admin-secret")

	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, access_token, refresh_token) VALUES (?, ?, ?);`, userID, "upstream-user-token", "upstream-refresh-token"); err != nil {
		t.Fatalf("seed sub2api auth token: %v", err)
	}

	body := `{"payment_event_id":"evt_api_key_1","user_id":` + strconv.FormatInt(userID, 10) + `,"api_key":{"name":"starter-key","group_id":77}}`
	req := httptest.NewRequest(http.MethodPost, "/admin/fulfillment/payment-success", bytes.NewReader([]byte(body)))
	req.Header.Set("Idempotency-Key", "idem-api-key-run")
	setBearerAuth(req, adminSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	var payload struct {
		Job struct {
			ID        int64  `json:"id"`
			Status    string `json:"status"`
			EventType string `json:"event_type"`
		} `json:"job"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Job.Status != "fulfilled" {
		t.Fatalf("expected fulfilled job after delegated api key creation, got %+v", payload.Job)
	}

	var finalStatus string
	if err := database.QueryRowContext(ctx, `SELECT status FROM als_fulfillment_jobs WHERE id = ?;`, payload.Job.ID).Scan(&finalStatus); err != nil {
		t.Fatalf("query final job status: %v", err)
	}
	if finalStatus != "fulfilled" {
		t.Fatalf("expected persisted status fulfilled, got %q", finalStatus)
	}
}

func TestAdminAvailableGroupsRequiresAdmin(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/admin/groups/all" {
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[]}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	_, userSessionToken := createUserViaAPI(t, mux, "groups-user@example.com", "Groups User", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "groups-admin@example.com", "Groups Admin", "admin", "test-admin-secret")

	unauthReq := httptest.NewRequest(http.MethodGet, "/admin/groups/available", nil)
	unauthRec := httptest.NewRecorder()
	mux.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated status %d, got %d", http.StatusUnauthorized, unauthRec.Code)
	}

	nonAdminReq := httptest.NewRequest(http.MethodGet, "/admin/groups/available", nil)
	setBearerAuth(nonAdminReq, userSessionToken)
	nonAdminRec := httptest.NewRecorder()
	mux.ServeHTTP(nonAdminRec, nonAdminReq)
	if nonAdminRec.Code != http.StatusForbidden {
		t.Fatalf("expected non-admin status %d, got %d", http.StatusForbidden, nonAdminRec.Code)
	}

	adminReq := httptest.NewRequest(http.MethodGet, "/admin/groups/available", nil)
	setBearerAuth(adminReq, adminSessionToken)
	adminRec := httptest.NewRecorder()
	mux.ServeHTTP(adminRec, adminReq)
	if adminRec.Code != http.StatusOK {
		t.Fatalf("expected admin status %d, got %d body=%s", http.StatusOK, adminRec.Code, adminRec.Body.String())
	}
}

func TestAdminAvailableGroupsReturnsMappedGroups(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/admin/groups/all" {
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
		if req.URL.Query().Get("platform") != "openai" {
			t.Fatalf("expected platform query openai, got %q", req.URL.Query().Get("platform"))
		}
		if got := req.Header.Get("X-Api-Key"); got != "admin-key-123" {
			t.Fatalf("expected admin api key, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[{"id":42,"name":"Starter Group","platform":"openai","subscription_type":"monthly"},{"id":77,"name":"Balance Group","platform":"openai","subscription_type":""}]}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	_, adminSessionToken := createUserViaAPI(t, mux, "groups-admin2@example.com", "Groups Admin Two", "admin", "test-admin-secret")

	req := httptest.NewRequest(http.MethodGet, "/admin/groups/available?platform=openai", nil)
	setBearerAuth(req, adminSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Groups []struct {
			ID               int64  `json:"id"`
			Name             string `json:"name"`
			Platform         string `json:"platform"`
			Type             string `json:"type"`
			SubscriptionType string `json:"subscription_type"`
		} `json:"groups"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(payload.Groups))
	}
	if payload.Groups[0].ID != 42 || payload.Groups[0].SubscriptionType != "monthly" || payload.Groups[0].Type != "monthly" {
		t.Fatalf("unexpected first group payload: %+v", payload.Groups[0])
	}
	if payload.Groups[1].ID != 77 || payload.Groups[1].SubscriptionType != "" {
		t.Fatalf("unexpected second group payload: %+v", payload.Groups[1])
	}
}

func TestAdminReplayFulfillmentJobRequiresAdminAndRetriesRetryableJob(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "starter", "Starter")
	insertTierGroupBinding(t, ctx, database, tierID, 11)

	balanceCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/admin/users/1/balance":
			balanceCalls++
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if got := req.Header.Get("Idempotency-Key"); got != "idem-replay-balance:balance" {
				t.Fatalf("unexpected idempotency key: %s", got)
			}
			if balanceCalls == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"code":503,"message":"temporary unavailable","reason":"IDEMPOTENCY_STORE_UNAVAILABLE"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":1,"balance":150}}`))
		default:
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	userID, userSessionToken := createUserViaAPI(t, mux, "replay-user@example.com", "Replay User", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "replay-admin@example.com", "Replay Admin", "admin", "test-admin-secret")
	if userID != 1 {
		t.Fatalf("expected first user id to be 1 for upstream assertion, got %d", userID)
	}
	insertActiveSubscription(t, ctx, database, userID, tierID, "2026-01-01T00:00:00Z")

	ingestBody := `{"payment_event_id":"evt_replay_balance_1","user_id":1,"balance_recharge":{"balance":50,"operation":"add","notes":"retry me"}}`
	ingestReq := httptest.NewRequest(http.MethodPost, "/admin/fulfillment/payment-success", bytes.NewBufferString(ingestBody))
	ingestReq.Header.Set("Content-Type", "application/json")
	ingestReq.Header.Set("Idempotency-Key", "idem-replay-balance")
	setBearerAuth(ingestReq, adminSessionToken)
	ingestRec := httptest.NewRecorder()
	mux.ServeHTTP(ingestRec, ingestReq)
	if ingestRec.Code != http.StatusAccepted {
		t.Fatalf("expected ingest status %d, got %d body=%s", http.StatusAccepted, ingestRec.Code, ingestRec.Body.String())
	}

	var ingestPayload adminPaymentSuccessResponse
	if err := json.NewDecoder(ingestRec.Body).Decode(&ingestPayload); err != nil {
		t.Fatalf("decode ingest replay response: %v", err)
	}
	if ingestPayload.Job.Status != fulfillment.StatusFailedRetryable {
		t.Fatalf("expected initial job status %q, got %q", fulfillment.StatusFailedRetryable, ingestPayload.Job.Status)
	}
	if balanceCalls != 1 {
		t.Fatalf("expected one upstream balance call after initial ingest, got %d", balanceCalls)
	}

	unauthReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/fulfillment/jobs/%d/replay", ingestPayload.Job.ID), nil)
	unauthRec := httptest.NewRecorder()
	mux.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated replay status %d, got %d body=%s", http.StatusUnauthorized, unauthRec.Code, unauthRec.Body.String())
	}

	userReplayReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/fulfillment/jobs/%d/replay", ingestPayload.Job.ID), nil)
	setBearerAuth(userReplayReq, userSessionToken)
	userReplayRec := httptest.NewRecorder()
	mux.ServeHTTP(userReplayRec, userReplayReq)
	if userReplayRec.Code != http.StatusForbidden {
		t.Fatalf("expected non-admin replay status %d, got %d body=%s", http.StatusForbidden, userReplayRec.Code, userReplayRec.Body.String())
	}

	replayReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/fulfillment/jobs/%d/replay", ingestPayload.Job.ID), nil)
	setBearerAuth(replayReq, adminSessionToken)
	replayRec := httptest.NewRecorder()
	mux.ServeHTTP(replayRec, replayReq)
	if replayRec.Code != http.StatusAccepted {
		t.Fatalf("expected replay status %d, got %d body=%s", http.StatusAccepted, replayRec.Code, replayRec.Body.String())
	}

	var replayPayload adminPaymentSuccessResponse
	if err := json.NewDecoder(replayRec.Body).Decode(&replayPayload); err != nil {
		t.Fatalf("decode replay response: %v", err)
	}
	if replayPayload.Job.Status != fulfillment.StatusFulfilled {
		t.Fatalf("expected replayed job status %q, got %q", fulfillment.StatusFulfilled, replayPayload.Job.Status)
	}
	if balanceCalls != 2 {
		t.Fatalf("expected two upstream balance calls after replay, got %d", balanceCalls)
	}

	storedJob, err := fulfillment.NewService(database).GetJobByID(ctx, ingestPayload.Job.ID)
	if err != nil {
		t.Fatalf("load stored replayed job: %v", err)
	}
	if storedJob.Status != fulfillment.StatusFulfilled {
		t.Fatalf("expected persisted replayed job status %q, got %q", fulfillment.StatusFulfilled, storedJob.Status)
	}

	events, err := fulfillment.NewService(database).ListEventsByJobID(ctx, ingestPayload.Job.ID)
	if err != nil {
		t.Fatalf("list replay events: %v", err)
	}
	foundReplayRequested := false
	for _, event := range events {
		if event.EventType == "admin_replay_requested" {
			foundReplayRequested = true
			break
		}
	}
	if !foundReplayRequested {
		t.Fatalf("expected replay event history to include admin_replay_requested, got %+v", events)
	}
}

func TestAdminReplayFulfillmentJobRejectsNonRetryableStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})
	userID, _ := createUserViaAPI(t, mux, "terminal-user@example.com", "Terminal User", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "replay-admin-2@example.com", "Replay Admin Two", "admin", "test-admin-secret")

	service := fulfillment.NewService(database)
	job, err := service.CreateOrLoadJobByIdempotency(ctx, &fulfillment.CreateJobInput{
		UserID:         &userID,
		EventType:      "payment_succeeded",
		PayloadJSON:    fmt.Sprintf(`{"payment_event_id":"evt-terminal","user_id":%d,"balance_recharge":{"balance":10,"operation":"add"}}`, userID),
		IdempotencyKey: "idem-terminal-replay",
	})
	if err != nil {
		t.Fatalf("create fulfillment job: %v", err)
	}

	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	errorMessage := "group not allowed"
	eventPayload := `{"error":"group not allowed"}`
	job, err = service.TransitionJob(ctx, job.ID, &fulfillment.TransitionInput{
		Status:       fulfillment.StatusFailedTerminal,
		ErrorMessage: &errorMessage,
		EventType:    "sub2api_api_key_create_failed_terminal",
		StartedAt:    &now,
		FinishedAt:   &now,
		RetryCount:   &job.RetryCount,
		EventPayload: &eventPayload,
	})
	if err != nil {
		t.Fatalf("transition fulfillment job: %v", err)
	}

	replayReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/fulfillment/jobs/%d/replay", job.ID), nil)
	setBearerAuth(replayReq, adminSessionToken)
	replayRec := httptest.NewRecorder()
	mux.ServeHTTP(replayRec, replayReq)
	if replayRec.Code != http.StatusBadRequest {
		t.Fatalf("expected replay rejection status %d, got %d body=%s", http.StatusBadRequest, replayRec.Code, replayRec.Body.String())
	}
	if !strings.Contains(replayRec.Body.String(), "only retryable fulfillment jobs can be replayed") {
		t.Fatalf("unexpected replay rejection body: %s", replayRec.Body.String())
	}
}

func TestAdminPackageLifecyclePersistsTierGroupBindings(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})
	_, adminSessionToken := createUserViaAPI(t, mux, "package-admin@example.com", "Package Admin", "admin", "test-admin-secret")

	createReq := httptest.NewRequest(http.MethodPost, "/admin/packages", bytes.NewReader([]byte(`{"code":"starter","name":"Starter Package","group_ids":[11,22]}`)))
	setBearerAuth(createReq, adminSessionToken)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d body=%s", http.StatusCreated, createRec.Code, createRec.Body.String())
	}

	var created struct {
		Code     string  `json:"code"`
		Name     string  `json:"name"`
		GroupIDs []int64 `json:"group_ids"`
	}
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Code != "starter" || created.Name != "Starter Package" {
		t.Fatalf("unexpected created package: %+v", created)
	}
	if len(created.GroupIDs) != 2 || created.GroupIDs[0] != 11 || created.GroupIDs[1] != 22 {
		t.Fatalf("unexpected created groups: %+v", created.GroupIDs)
	}

	var bindingCount int64
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_tier_group_bindings WHERE tier_id = (SELECT id FROM als_tiers WHERE code = ?);`, "starter").Scan(&bindingCount); err != nil {
		t.Fatalf("count tier group bindings: %v", err)
	}
	if bindingCount != 2 {
		t.Fatalf("expected 2 tier group bindings, got %d", bindingCount)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/admin/packages", nil)
	setBearerAuth(listReq, adminSessionToken)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d body=%s", http.StatusOK, listRec.Code, listRec.Body.String())
	}

	var listPayload struct {
		Packages []struct {
			Code     string  `json:"code"`
			Name     string  `json:"name"`
			GroupIDs []int64 `json:"group_ids"`
		} `json:"packages"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listPayload.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(listPayload.Packages))
	}
	if listPayload.Packages[0].Code != "starter" || len(listPayload.Packages[0].GroupIDs) != 2 {
		t.Fatalf("unexpected listed package: %+v", listPayload.Packages[0])
	}

	getReq := httptest.NewRequest(http.MethodGet, "/admin/packages/starter", nil)
	setBearerAuth(getReq, adminSessionToken)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d body=%s", http.StatusOK, getRec.Code, getRec.Body.String())
	}

	var getPayload struct {
		Code     string  `json:"code"`
		Name     string  `json:"name"`
		GroupIDs []int64 `json:"group_ids"`
	}
	if err := json.NewDecoder(getRec.Body).Decode(&getPayload); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if getPayload.Code != "starter" || getPayload.Name != "Starter Package" {
		t.Fatalf("unexpected get payload: %+v", getPayload)
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/admin/packages/starter", bytes.NewReader([]byte(`{"name":"Starter Plus","group_ids":[22,33]}`)))
	setBearerAuth(updateReq, adminSessionToken)
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update status %d, got %d body=%s", http.StatusOK, updateRec.Code, updateRec.Body.String())
	}

	var updated struct {
		Code     string  `json:"code"`
		Name     string  `json:"name"`
		GroupIDs []int64 `json:"group_ids"`
	}
	if err := json.NewDecoder(updateRec.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.Name != "Starter Plus" {
		t.Fatalf("expected updated name, got %+v", updated)
	}
	if len(updated.GroupIDs) != 2 || updated.GroupIDs[0] != 22 || updated.GroupIDs[1] != 33 {
		t.Fatalf("unexpected updated groups: %+v", updated.GroupIDs)
	}

	rows, err := database.QueryContext(ctx, `SELECT group_id FROM als_tier_group_bindings WHERE tier_id = (SELECT id FROM als_tiers WHERE code = ?) ORDER BY group_id ASC;`, "starter")
	if err != nil {
		t.Fatalf("query updated bindings: %v", err)
	}
	defer rows.Close()
	bindings := make([]int64, 0)
	for rows.Next() {
		var groupID int64
		if err := rows.Scan(&groupID); err != nil {
			t.Fatalf("scan binding: %v", err)
		}
		bindings = append(bindings, groupID)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate bindings: %v", err)
	}
	if len(bindings) != 2 || bindings[0] != 22 || bindings[1] != 33 {
		t.Fatalf("unexpected persisted bindings after update: %+v", bindings)
	}
}

func TestAdminPackageValidationAndNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	insertTier(t, ctx, database, "existing", "Existing Package")
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})
	_, adminSessionToken := createUserViaAPI(t, mux, "package-admin2@example.com", "Package Admin 2", "admin", "test-admin-secret")

	badCreateReq := httptest.NewRequest(http.MethodPost, "/admin/packages", bytes.NewReader([]byte(`{"name":"Missing Code","group_ids":[11]}`)))
	setBearerAuth(badCreateReq, adminSessionToken)
	badCreateRec := httptest.NewRecorder()
	mux.ServeHTTP(badCreateRec, badCreateReq)
	if badCreateRec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad create status %d, got %d", http.StatusBadRequest, badCreateRec.Code)
	}

	duplicateReq := httptest.NewRequest(http.MethodPost, "/admin/packages", bytes.NewReader([]byte(`{"code":"existing","name":"Duplicate","group_ids":[11]}`)))
	setBearerAuth(duplicateReq, adminSessionToken)
	duplicateRec := httptest.NewRecorder()
	mux.ServeHTTP(duplicateRec, duplicateReq)
	if duplicateRec.Code != http.StatusBadRequest {
		t.Fatalf("expected duplicate create status %d, got %d body=%s", http.StatusBadRequest, duplicateRec.Code, duplicateRec.Body.String())
	}

	missingReq := httptest.NewRequest(http.MethodGet, "/admin/packages/missing", nil)
	setBearerAuth(missingReq, adminSessionToken)
	missingRec := httptest.NewRecorder()
	mux.ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("expected missing package status %d, got %d", http.StatusNotFound, missingRec.Code)
	}

	badUpdateReq := httptest.NewRequest(http.MethodPut, "/admin/packages/existing", bytes.NewReader([]byte(`{"name":"","group_ids":[]}`)))
	setBearerAuth(badUpdateReq, adminSessionToken)
	badUpdateRec := httptest.NewRecorder()
	mux.ServeHTTP(badUpdateRec, badUpdateReq)
	if badUpdateRec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad update status %d, got %d", http.StatusBadRequest, badUpdateRec.Code)
	}
}

func TestUserVisibleGroupsAreFilteredByPackageBindings(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "starter", "Starter")
	insertTierGroupBinding(t, ctx, database, tierID, 11)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/groups/available" {
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":11,"name":"Starter Group","code":"starter-group","platform":"openai","status":"active"},{"id":22,"name":"Pro Group","code":"pro-group","platform":"openai","status":"active"}]}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithHTTPClient(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	userID, userSessionToken := createUserViaAPI(t, mux, "groups-user@example.com", "Groups User", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "groups-admin@example.com", "Groups Admin", "admin", "test-admin-secret")
	insertActiveSubscription(t, ctx, database, userID, tierID, "2026-01-01T00:00:00Z")

	userReq := httptest.NewRequest(http.MethodGet, "/groups/available", nil)
	setBearerAuth(userReq, userSessionToken)
	userRec := httptest.NewRecorder()
	mux.ServeHTTP(userRec, userReq)
	if userRec.Code != http.StatusOK {
		t.Fatalf("expected user status %d, got %d body=%s", http.StatusOK, userRec.Code, userRec.Body.String())
	}

	var userPayload struct {
		Data []struct {
			ID   int64  `json:"id"`
			Code string `json:"code"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(userRec.Body).Decode(&userPayload); err != nil {
		t.Fatalf("decode user groups response: %v", err)
	}
	if len(userPayload.Data) != 1 || userPayload.Data[0].Code != "starter-group" {
		t.Fatalf("unexpected filtered groups payload: %+v", userPayload.Data)
	}

	adminReq := httptest.NewRequest(http.MethodGet, "/groups/available", nil)
	setBearerAuth(adminReq, adminSessionToken)
	adminRec := httptest.NewRecorder()
	mux.ServeHTTP(adminRec, adminReq)
	if adminRec.Code != http.StatusOK {
		t.Fatalf("expected admin status %d, got %d body=%s", http.StatusOK, adminRec.Code, adminRec.Body.String())
	}
	var adminPayload struct {
		Data []struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	if err := json.NewDecoder(adminRec.Body).Decode(&adminPayload); err != nil {
		t.Fatalf("decode admin groups response: %v", err)
	}
	if len(adminPayload.Data) != 2 {
		t.Fatalf("expected admin to see 2 groups, got %d", len(adminPayload.Data))
	}
}

func TestUserAPIKeysAreFilteredAndDetailForbiddenAcrossGroups(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "starter", "Starter")
	insertTierGroupBinding(t, ctx, database, tierID, 11)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"data":[{"id":11,"name":"Starter Group","code":"starter-group","platform":"openai","status":"active"},{"id":22,"name":"Pro Group","code":"pro-group","platform":"openai","status":"active"}]}`))
		case "/api/v1/api-keys":
			_, _ = w.Write([]byte(`{"data":{"data":[{"id":101,"name":"starter-key","key":"sk-starter","group_id":11,"group":{"id":11,"name":"Starter Group"},"status":"active","quota":0,"quota_used":0,"expires_at":"","created_at":"2026-01-01T00:00:00Z"},{"id":202,"name":"pro-key","key":"sk-pro","group_id":22,"group":{"id":22,"name":"Pro Group"},"status":"active","quota":0,"quota_used":0,"expires_at":"","created_at":"2026-01-01T00:00:00Z"}],"total":2,"page":1,"per_page":20}}`))
		case "/api/v1/api-keys/101":
			_, _ = w.Write([]byte(`{"data":{"id":101,"name":"starter-key","key":"sk-starter","group_id":11,"group":{"id":11,"name":"Starter Group"},"status":"active"}}`))
		case "/api/v1/api-keys/202":
			_, _ = w.Write([]byte(`{"data":{"id":202,"name":"pro-key","key":"sk-pro","group_id":22,"group":{"id":22,"name":"Pro Group"},"status":"active"}}`))
		default:
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithHTTPClient(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	userID, userSessionToken := createUserViaAPI(t, mux, "keys-user@example.com", "Keys User", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "keys-admin@example.com", "Keys Admin", "admin", "test-admin-secret")
	insertActiveSubscription(t, ctx, database, userID, tierID, "2026-01-01T00:00:00Z")

	listReq := httptest.NewRequest(http.MethodGet, "/api-keys?page=1", nil)
	setBearerAuth(listReq, userSessionToken)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d body=%s", http.StatusOK, listRec.Code, listRec.Body.String())
	}

	var listPayload struct {
		Data struct {
			Data []struct {
				ID      int64 `json:"id"`
				GroupID int64 `json:"group_id"`
			} `json:"data"`
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode api keys response: %v", err)
	}
	if len(listPayload.Data.Data) != 1 || listPayload.Data.Data[0].ID != 101 || listPayload.Data.Data[0].GroupID != 11 {
		t.Fatalf("unexpected filtered key list: %+v", listPayload.Data.Data)
	}
	if listPayload.Data.Total != 1 {
		t.Fatalf("expected filtered total 1, got %d", listPayload.Data.Total)
	}

	allowedDetailReq := httptest.NewRequest(http.MethodGet, "/api-keys/101", nil)
	setBearerAuth(allowedDetailReq, userSessionToken)
	allowedDetailRec := httptest.NewRecorder()
	mux.ServeHTTP(allowedDetailRec, allowedDetailReq)
	if allowedDetailRec.Code != http.StatusOK {
		t.Fatalf("expected allowed detail status %d, got %d body=%s", http.StatusOK, allowedDetailRec.Code, allowedDetailRec.Body.String())
	}

	blockedDetailReq := httptest.NewRequest(http.MethodGet, "/api-keys/202", nil)
	setBearerAuth(blockedDetailReq, userSessionToken)
	blockedDetailRec := httptest.NewRecorder()
	mux.ServeHTTP(blockedDetailRec, blockedDetailReq)
	if blockedDetailRec.Code != http.StatusForbidden {
		t.Fatalf("expected blocked detail status %d, got %d body=%s", http.StatusForbidden, blockedDetailRec.Code, blockedDetailRec.Body.String())
	}

	adminDetailReq := httptest.NewRequest(http.MethodGet, "/api-keys/202", nil)
	setBearerAuth(adminDetailReq, adminSessionToken)
	adminDetailRec := httptest.NewRecorder()
	mux.ServeHTTP(adminDetailRec, adminDetailReq)
	if adminDetailRec.Code != http.StatusOK {
		t.Fatalf("expected admin detail status %d, got %d body=%s", http.StatusOK, adminDetailRec.Code, adminDetailRec.Body.String())
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
	database, err := db.Open(ctx, "sqlite", dbFile)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := db.ApplyMigrations(ctx, database, "sqlite"); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}

	return database
}

func insertTier(t *testing.T, ctx context.Context, database *sql.DB, code, name string) int64 {
	t.Helper()

	result, err := database.ExecContext(ctx, `INSERT INTO als_tiers(code, name) VALUES (?, ?);`, code, name)
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

	result, err := database.ExecContext(ctx, `INSERT INTO als_service_items(code, name, unit) VALUES (?, ?, ?);`, code, name, unit)
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
		INSERT INTO als_tier_default_items(tier_id, service_item_id, included_units)
		VALUES (?, ?, ?);
	`, tierID, serviceItemID, includedUnits)
	if err != nil {
		t.Fatalf("insert tier default item error = %v", err)
	}
}

func insertTierGroupBinding(t *testing.T, ctx context.Context, database *sql.DB, tierID int64, groupID int64) {
	t.Helper()

	_, err := database.ExecContext(ctx, `INSERT INTO als_tier_group_bindings(tier_id, group_id) VALUES (?, ?);`, tierID, groupID)
	if err != nil {
		t.Fatalf("insert tier group binding error = %v", err)
	}
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func stripeTestSignature(secret string, timestamp int64, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	return fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
}

func insertUnitPrice(t *testing.T, ctx context.Context, database *sql.DB, serviceItemID int64, tierID any, pricePerUnitMicros int64, currency, effectiveFrom string) {
	t.Helper()

	_, err := database.ExecContext(ctx, `
		INSERT INTO als_unit_prices(service_item_id, tier_id, price_per_unit_micros, currency, effective_from)
		VALUES (?, ?, ?, ?, ?);
	`, serviceItemID, tierID, pricePerUnitMicros, currency, effectiveFrom)
	if err != nil {
		t.Fatalf("insert unit price error = %v", err)
	}
}

func TestAdminPackageCRUDWithNewFields(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})
	_, adminSessionToken := createUserViaAPI(t, mux, "pkg-new-fields@example.com", "Pkg Admin", "admin", "test-admin-secret")

	// Create package with all new fields
	createBody, _ := json.Marshal(map[string]any{
		"code":          "pro-monthly",
		"name":          "Pro Monthly",
		"group_ids":     []int64{11, 22},
		"price_micros":  29900000,
		"value_type":    "days",
		"value_amount":  30,
		"description":   "Enhanced speed for dedicated developers.",
		"features_json": `["10 Global Nodes","500GB Monthly Traffic","Priority Email Support"]`,
		"is_enabled":    true,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/admin/packages", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	setBearerAuth(createReq, adminSessionToken)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, createRec.Code, createRec.Body.String())
	}

	var created struct {
		Code        string  `json:"code"`
		Name        string  `json:"name"`
		GroupIDs    []int64 `json:"group_ids"`
		PriceMicros int64   `json:"price_micros"`
		ValueType   string  `json:"value_type"`
		ValueAmount int64   `json:"value_amount"`
		Description string  `json:"description"`
		Features    []string `json:"features"`
		IsEnabled   bool    `json:"is_enabled"`
	}
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Code != "pro-monthly" {
		t.Fatalf("expected code pro-monthly, got %s", created.Code)
	}
	if created.PriceMicros != 29900000 {
		t.Fatalf("expected price_micros 29900000, got %d", created.PriceMicros)
	}
	if created.ValueType != "days" {
		t.Fatalf("expected value_type days, got %s", created.ValueType)
	}
	if created.ValueAmount != 30 {
		t.Fatalf("expected value_amount 30, got %d", created.ValueAmount)
	}
	if created.Description != "Enhanced speed for dedicated developers." {
		t.Fatalf("unexpected description: %s", created.Description)
	}
	if len(created.Features) != 3 {
		t.Fatalf("expected 3 features, got %d: %+v", len(created.Features), created.Features)
	}
	if !created.IsEnabled {
		t.Fatal("expected is_enabled true")
	}

	// List packages — should include new fields
	listReq := httptest.NewRequest(http.MethodGet, "/admin/packages", nil)
	setBearerAuth(listReq, adminSessionToken)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d", http.StatusOK, listRec.Code)
	}

	var listPayload struct {
		Packages []struct {
			Code        string   `json:"code"`
			PriceMicros int64    `json:"price_micros"`
			ValueType   string   `json:"value_type"`
			Features    []string `json:"features"`
			IsEnabled   bool     `json:"is_enabled"`
		} `json:"packages"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	found := false
	for _, pkg := range listPayload.Packages {
		if pkg.Code == "pro-monthly" {
			found = true
			if pkg.PriceMicros != 29900000 {
				t.Fatalf("list: expected price_micros 29900000, got %d", pkg.PriceMicros)
			}
			break
		}
	}
	if !found {
		t.Fatal("pro-monthly not found in list")
	}

	// Update package
	updateBody, _ := json.Marshal(map[string]any{
		"name":          "Pro Monthly Updated",
		"group_ids":     []int64{11},
		"price_micros":  19900000,
		"value_type":    "days",
		"value_amount":  90,
		"description":   "Better deal.",
		"features_json": `["20 Global Nodes","Unlimited Traffic"]`,
		"is_enabled":    false,
	})
	updateReq := httptest.NewRequest(http.MethodPut, "/admin/packages/pro-monthly", bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	setBearerAuth(updateReq, adminSessionToken)
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update status %d, got %d: %s", http.StatusOK, updateRec.Code, updateRec.Body.String())
	}

	var updated struct {
		PriceMicros int64    `json:"price_micros"`
		ValueAmount int64    `json:"value_amount"`
		IsEnabled   bool     `json:"is_enabled"`
		Features    []string `json:"features"`
	}
	if err := json.NewDecoder(updateRec.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.PriceMicros != 19900000 {
		t.Fatalf("expected updated price_micros 19900000, got %d", updated.PriceMicros)
	}
	if updated.ValueAmount != 90 {
		t.Fatalf("expected updated value_amount 90, got %d", updated.ValueAmount)
	}
	if updated.IsEnabled {
		t.Fatal("expected is_enabled false after update")
	}
	if len(updated.Features) != 2 {
		t.Fatalf("expected 2 features after update, got %d", len(updated.Features))
	}
}

func TestPublicPackagesReturnsOnlyEnabled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)

	// Insert als_tiers directly — one enabled, one disabled
	_, err := database.ExecContext(ctx, `INSERT INTO als_tiers(code, name, price_micros, value_type, value_amount, description, features_json, is_enabled) VALUES ('free', 'Free', 0, 'days', 30, 'Perfect for exploring.', '["2 Global Nodes"]', 1);`)
	if err != nil {
		t.Fatalf("insert free tier: %v", err)
	}
	_, err = database.ExecContext(ctx, `INSERT INTO als_tiers(code, name, price_micros, value_type, value_amount, description, features_json, is_enabled) VALUES ('hidden', 'Hidden', 99000000, 'days', 365, 'Hidden tier.', '["Everything"]', 0);`)
	if err != nil {
		t.Fatalf("insert hidden tier: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	req := httptest.NewRequest(http.MethodGet, "/public/packages", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Packages []struct {
			Code string `json:"code"`
		} `json:"packages"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Packages) != 1 {
		t.Fatalf("expected 1 public package, got %d", len(payload.Packages))
	}
	if payload.Packages[0].Code != "free" {
		t.Fatalf("expected package code 'free', got %s", payload.Packages[0].Code)
	}
}
