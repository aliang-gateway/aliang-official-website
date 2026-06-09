package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/fulfillment"
	"ai-api-portal/backend/internal/proxy"
	portalstripe "ai-api-portal/backend/internal/stripe"
	"ai-api-portal/backend/internal/user"
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
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.URL.Path == "/api/v1/admin/groups/all":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[{"id":77,"name":"Pro Subscription","subscription_type":"monthly"}]}`))
		case req.URL.Path == "/api/v1/admin/users/9001/subscriptions" && req.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[]}`))
		case req.URL.Path == "/api/v1/admin/subscriptions/assign":
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if got := req.Header.Get("Idempotency-Key"); got != "stripe:evt_stripe_1:assign-subscription:77" {
				t.Fatalf("unexpected idempotency key: %q", got)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read upstream body: %v", err)
			}
			if !strings.Contains(string(body), `"user_id":9001`) || !strings.Contains(string(body), `"group_id":77`) || !strings.Contains(string(body), `"validity_days":30`) {
				t.Fatalf("unexpected assign subscription payload: %s", string(body))
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":1,"user_id":9001,"group_id":77,"status":"active","validity_days":30}}`))
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"data":[],"total":0,"page":1,"per_page":100}}`))
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodPost:
			if got := req.Header.Get("Idempotency-Key"); got != "stripe:evt_stripe_1:ensure-key:77" {
				t.Fatalf("unexpected api key idempotency key: %q", got)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"id":91,"name":"auto-key","key":"sk-test-auto","group_id":77,"status":"active"}}`))
		default:
			t.Fatalf("unexpected upstream path: %s %s", req.Method, req.URL.Path)
		}
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
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, upstream_user_id, access_token, refresh_token) VALUES (?, ?, ?, ?);`, userID, 9001, "upstream-user-token", "upstream-refresh-token"); err != nil {
		t.Fatalf("seed sub2api auth token: %v", err)
	}

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
		recordStatus    string
		recordEventID   string
		recordTierCode  string
		recordFulfillID sql.NullInt64
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

func TestCheckoutStatusAutoRetriesRetryableFulfillment(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "daily", "3日体验")
	if _, err := database.ExecContext(ctx, `UPDATE als_tiers SET price_micros = 1000000, value_type = 'days', value_amount = 3, is_enabled = 1 WHERE id = ?;`, tierID); err != nil {
		t.Fatalf("update tier fields: %v", err)
	}
	insertTierGroupBinding(t, ctx, database, tierID, 77)

	callCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.URL.Path == "/api/v1/admin/groups/all":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[{"id":77,"name":"Daily Subscription","subscription_type":"daily"}]}`))
		case req.URL.Path == "/api/v1/admin/users/1/subscriptions" && req.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[]}`))
		case req.URL.Path == "/api/v1/admin/subscriptions/assign":
			callCount++
			if callCount == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message":"internal error"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":2,"user_id":1,"group_id":77,"status":"active"}}`))
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"data":[],"total":0,"page":1,"per_page":100}}`))
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"id":91,"name":"auto-key","key":"sk-test-auto","group_id":77,"status":"active"}}`))
		default:
			t.Fatalf("unexpected upstream path: %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{
		AdminBootstrapSecret: "test-admin-secret",
		ProxyClient:          proxyClient,
	})
	userID, userSessionToken := createUserViaAPI(t, mux, "retry-status-user@example.com", "Retry Status User", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "retry-status-admin@example.com", "Retry Status Admin", "admin", "test-admin-secret")
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, access_token, refresh_token) VALUES (?, ?, ?);`, userID, "upstream-user-token", "upstream-refresh-token"); err != nil {
		t.Fatalf("seed sub2api auth token: %v", err)
	}

	body := `{"payment_event_id":"evt_retry_status_1","provider":"stripe","user_id":` + strconv.FormatInt(userID, 10) + `,"tier_code":"daily","order_id":"cs_auto_retry"}`
	createReq := httptest.NewRequest(http.MethodPost, "/admin/fulfillment/payment-success", bytes.NewReader([]byte(body)))
	createReq.Header.Set("Idempotency-Key", "stripe:evt_retry_status_1")
	setBearerAuth(createReq, adminSessionToken)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusAccepted {
		t.Fatalf("expected create status %d, got %d body=%s", http.StatusAccepted, createRec.Code, createRec.Body.String())
	}

	var createPayload struct {
		Job struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"job"`
	}
	if err := json.NewDecoder(createRec.Body).Decode(&createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createPayload.Job.Status != fulfillment.StatusFailedRetryable {
		t.Fatalf("expected initial retryable job, got %+v", createPayload.Job)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO als_payment_records(provider, checkout_session_id, payment_event_id, user_id, tier_code, package_name, amount_minor, currency, status, fulfillment_job_id, payload_json)
		VALUES ('stripe', 'cs_auto_retry', 'evt_retry_status_1', ?, 'daily', '3日体验', 1000, 'cny', 'payment_succeeded', ?, '{}')
		ON CONFLICT (checkout_session_id) DO NOTHING;
	`, userID, createPayload.Job.ID); err != nil {
		t.Fatalf("insert payment record: %v", err)
	}

	if _, err := database.ExecContext(ctx, `UPDATE als_fulfillment_jobs SET available_at = CURRENT_TIMESTAMP WHERE id = ?;`, createPayload.Job.ID); err != nil {
		t.Fatalf("update job available_at: %v", err)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/checkout/package/status?session_id=cs_auto_retry", nil)
	setBearerAuth(statusReq, userSessionToken)
	statusRec := httptest.NewRecorder()
	mux.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected status endpoint %d, got %d body=%s", http.StatusOK, statusRec.Code, statusRec.Body.String())
	}

	var statusPayload struct {
		Status         string `json:"status"`
		FulfillmentJob struct {
			Status string `json:"status"`
		} `json:"fulfillment_job"`
	}
	if err := json.NewDecoder(statusRec.Body).Decode(&statusPayload); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if statusPayload.Status != "fulfilled" || statusPayload.FulfillmentJob.Status != fulfillment.StatusFulfilled {
		t.Fatalf("expected auto-retried fulfilled status, got %+v", statusPayload)
	}
	if callCount != 2 {
		t.Fatalf("expected two upstream calls after auto retry, got %d", callCount)
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

func TestPackageBalanceFulfillmentUsesAdminBalanceUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "credit-plus", "Credit Plus")
	if _, err := database.ExecContext(ctx, `UPDATE als_tiers SET price_micros = 25000000, value_type = 'balance', value_amount = 25000000, is_enabled = 1, is_published = 1 WHERE id = ?;`, tierID); err != nil {
		t.Fatalf("update tier fields: %v", err)
	}
	insertTierGroupBinding(t, ctx, database, tierID, 88)

	var upstreamCalls []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.URL.Path == "/api/v1/admin/groups/all":
			upstreamCalls = append(upstreamCalls, "groups")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[{"id":88,"name":"Credit Group","subscription_type":"standard"}]}`))
		case req.URL.Path == "/api/v1/admin/users/1/balance":
			upstreamCalls = append(upstreamCalls, "balance")
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST balance update, got %s", req.Method)
			}
			if got := req.Header.Get("Idempotency-Key"); got != "idem-package-balance:package:balance" {
				t.Fatalf("expected balance package idempotency key, got %q", got)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read balance payload: %v", err)
			}
			if !strings.Contains(string(body), `"balance":25`) || !strings.Contains(string(body), `"operation":"add"`) {
				t.Fatalf("unexpected balance payload: %s", string(body))
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":1,"balance":25}}`))
		case req.URL.Path == "/api/v1/admin/users/1" && req.Method == http.MethodGet:
			upstreamCalls = append(upstreamCalls, "get-user")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":1,"email":"credit-user@example.com","allowed_groups":[]}}`))
		case req.URL.Path == "/api/v1/admin/users/1" && req.Method == http.MethodPut:
			upstreamCalls = append(upstreamCalls, "grant-group")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":1,"email":"credit-user@example.com","allowed_groups":[88]}}`))
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodGet:
			upstreamCalls = append(upstreamCalls, "list-key")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"data":[],"total":0,"page":1,"per_page":100}}`))
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodPost:
			upstreamCalls = append(upstreamCalls, "key")
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"id":91,"name":"auto-key","key":"sk-test-auto","group_id":88,"status":"active"}}`))
		case req.URL.Path == "/api/v1/admin/redeem-codes/create-and-redeem":
			t.Fatalf("package balance fulfillment must not use redeem-codes")
		default:
			t.Fatalf("unexpected upstream path: %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	userID, _ := createUserViaAPI(t, mux, "credit-user@example.com", "Credit User", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "credit-admin@example.com", "Credit Admin", "admin", "test-admin-secret")
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, upstream_user_id, access_token, refresh_token) VALUES (?, ?, ?, ?);`, userID, 1, "upstream-user-token", "upstream-refresh-token"); err != nil {
		t.Fatalf("seed sub2api auth token: %v", err)
	}

	body := `{"payment_event_id":"evt_package_balance","user_id":` + strconv.FormatInt(userID, 10) + `,"tier_code":"credit-plus","order_id":"cs_package_balance"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/fulfillment/payment-success", bytes.NewReader([]byte(body)))
	req.Header.Set("Idempotency-Key", "idem-package-balance")
	setBearerAuth(req, adminSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusAccepted, rec.Code, rec.Body.String())
	}
	if got := strings.Join(upstreamCalls, ","); got != "groups,balance,get-user,grant-group,list-key,key" {
		t.Fatalf("expected upstream calls groups,balance,get-user,grant-group,list-key,key; got %s", got)
	}
}

func TestAdminPaymentSuccessExecutesDelegatedAPIKeyCreationImmediately(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/api-keys" {
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

func TestAdminQuickCreateUserRegistersUpstreamAndStoresToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)

	var upstreamEmail string
	var upstreamName string
	var upstreamPassword string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/auth/register" {
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		var payload struct {
			Email    string `json:"email"`
			Name     string `json:"name"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode upstream register request: %v", err)
		}
		upstreamEmail = payload.Email
		upstreamName = payload.Name
		upstreamPassword = payload.Password
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"quick-upstream-access","refresh_token":"quick-upstream-refresh","user":{"id":901,"email":"quick-user@example.com","role":"user"}}}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	_, adminSessionToken := createUserViaAPI(t, mux, "quick-admin@example.com", "Quick Admin", "admin", "test-admin-secret")

	req := httptest.NewRequest(http.MethodPost, "/admin/users/quick-create", bytes.NewReader([]byte(`{"email":"Quick-User@Example.com"}`)))
	req.Header.Set("Content-Type", "application/json")
	setBearerAuth(req, adminSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var response adminQuickCreateUserResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode quick-create response: %v", err)
	}
	if response.ID != 901 || response.Email != "quick-user@example.com" || response.Name != "quick-user" || response.Password == "" {
		t.Fatalf("unexpected quick-create response: %+v", response)
	}
	if upstreamEmail != response.Email || upstreamName != response.Name || upstreamPassword != response.Password {
		t.Fatalf("unexpected upstream register payload email=%q name=%q password=%q response=%+v", upstreamEmail, upstreamName, upstreamPassword, response)
	}

	var (
		passwordHash     sql.NullString
		emailVerified    bool
		accessToken      string
		refreshToken     sql.NullString
		upstreamUserID   sql.NullInt64
		localWalletCount int64
	)
	if err := database.QueryRowContext(ctx, `SELECT password_hash, email_verified FROM als_users WHERE id = ?;`, response.ID).Scan(&passwordHash, &emailVerified); err != nil {
		t.Fatalf("query created user: %v", err)
	}
	if !passwordHash.Valid || !user.CheckPassword(response.Password, passwordHash.String) {
		t.Fatalf("expected local password hash to match generated password")
	}
	if !emailVerified {
		t.Fatalf("expected admin-created local user to be email verified")
	}
	if err := database.QueryRowContext(ctx, `
		SELECT access_token, refresh_token, upstream_user_id
		FROM als_sub2api_auth_tokens
		WHERE user_id = ?;
	`, response.ID).Scan(&accessToken, &refreshToken, &upstreamUserID); err != nil {
		t.Fatalf("query stored sub2api token: %v", err)
	}
	if accessToken != "quick-upstream-access" || !refreshToken.Valid || refreshToken.String != "quick-upstream-refresh" || !upstreamUserID.Valid || upstreamUserID.Int64 != 901 {
		t.Fatalf("unexpected stored sub2api token access=%q refresh=%+v upstream=%+v", accessToken, refreshToken, upstreamUserID)
	}
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_user_wallets WHERE user_id = ?;`, response.ID).Scan(&localWalletCount); err != nil {
		t.Fatalf("count wallet rows: %v", err)
	}
	if localWalletCount != 1 {
		t.Fatalf("expected wallet row for created user, got %d", localWalletCount)
	}
}

func TestDistributorQuickCreateUserBindsCreatedUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)

	var upstreamEmail string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/auth/register" {
			t.Fatalf("unexpected upstream path: %s", req.URL.Path)
		}
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		var payload struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode upstream register request: %v", err)
		}
		upstreamEmail = payload.Email
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"distributor-upstream-access","refresh_token":"distributor-upstream-refresh","user":{"id":902,"email":"new-distributor-user@example.com","role":"user"}}}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	distributorID, distributorSessionToken := createUserViaAPI(t, mux, "quick-distributor@example.com", "Quick Distributor", "distributor", "test-admin-secret")

	req := httptest.NewRequest(http.MethodPost, "/distributor/users/quick-create", bytes.NewReader([]byte(`{"email":"New-Distributor-User@Example.com"}`)))
	req.Header.Set("Content-Type", "application/json")
	setBearerAuth(req, distributorSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected distributor quick-create status %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}
	var response adminQuickCreateUserResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode distributor quick-create response: %v", err)
	}
	if response.ID != 902 || response.DistributorBindingID <= 0 || response.Email != "new-distributor-user@example.com" || response.Password == "" {
		t.Fatalf("unexpected distributor quick-create response: %+v", response)
	}
	if upstreamEmail != response.Email {
		t.Fatalf("expected upstream email %q, got %q", response.Email, upstreamEmail)
	}

	var (
		bindingDistributorID int64
		bindingSource        string
		accessToken          string
		upstreamUserID       sql.NullInt64
	)
	if err := database.QueryRowContext(ctx, `
		SELECT distributor_user_id, source
		FROM als_distributor_user_bindings
		WHERE user_id = ?;
	`, response.ID).Scan(&bindingDistributorID, &bindingSource); err != nil {
		t.Fatalf("query distributor binding: %v", err)
	}
	if bindingDistributorID != distributorID || bindingSource != "distributor_quick_create" {
		t.Fatalf("unexpected distributor binding distributor=%d source=%q", bindingDistributorID, bindingSource)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT access_token, upstream_user_id
		FROM als_sub2api_auth_tokens
		WHERE user_id = ?;
	`, response.ID).Scan(&accessToken, &upstreamUserID); err != nil {
		t.Fatalf("query distributor-created token: %v", err)
	}
	if accessToken != "distributor-upstream-access" || !upstreamUserID.Valid || upstreamUserID.Int64 != 902 {
		t.Fatalf("unexpected distributor-created token access=%q upstream=%+v", accessToken, upstreamUserID)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/distributor/users", nil)
	setBearerAuth(listReq, distributorSessionToken)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected distributor users status %d, got %d body=%s", http.StatusOK, listRec.Code, listRec.Body.String())
	}
	var listPayload listDistributorUsersResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode distributor users: %v", err)
	}
	if len(listPayload.Users) != 1 || listPayload.Users[0].UserID != response.ID || listPayload.Users[0].Email != response.Email {
		t.Fatalf("expected created user to be visible to distributor, got %+v", listPayload.Users)
	}
}

func TestAdminAssignPackageAcceptsUpstreamUserID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "pro-monthly", "Pro Monthly")
	if _, err := database.ExecContext(ctx, `UPDATE als_tiers SET price_micros = 29900000, value_type = 'days', value_amount = 30, is_enabled = 1 WHERE id = ?;`, tierID); err != nil {
		t.Fatalf("update tier fields: %v", err)
	}
	insertTierGroupBinding(t, ctx, database, tierID, 77)

	var (
		assignedUserID int64
		upstreamCalls  []string
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.URL.Path == "/api/v1/auth/login":
			upstreamCalls = append(upstreamCalls, "login")
			var loginPayload struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			if err := json.NewDecoder(req.Body).Decode(&loginPayload); err != nil {
				t.Fatalf("decode login payload: %v", err)
			}
			if loginPayload.Email != "assign-upstream-user@example.com" || loginPayload.Password != "LatestPassword#123" {
				t.Fatalf("unexpected login payload: %+v", loginPayload)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"new-upstream-user-token","refresh_token":"new-upstream-refresh-token","user":{"id":32,"email":"assign-upstream-user@example.com"}}}`))
		case req.URL.Path == "/api/v1/admin/groups/all":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[{"id":77,"name":"Pro Subscription","subscription_type":"monthly"}]}`))
		case req.URL.Path == "/api/v1/admin/users/32/subscriptions" && req.Method == http.MethodGet:
			upstreamCalls = append(upstreamCalls, "list-subscriptions")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[]}`))
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodGet:
			upstreamCalls = append(upstreamCalls, "list-key")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"data":[],"total":0,"page":1,"per_page":100}}`))
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodPost:
			upstreamCalls = append(upstreamCalls, "key")
			var payload proxy.CreateUserAPIKeyRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode api key payload: %v", err)
			}
			if payload.Name != "auto-key" || payload.GroupID != 77 {
				t.Fatalf("unexpected api key payload: %+v", payload)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"id":91,"name":"auto-key","key":"sk-test-auto","group_id":77,"status":"active"}}`))
		case req.URL.Path == "/api/v1/admin/subscriptions/assign":
			upstreamCalls = append(upstreamCalls, "assign")
			var payload proxy.AssignAdminSubscriptionRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode assign subscription payload: %v", err)
			}
			assignedUserID = payload.UserID
			if payload.UserID != 32 || payload.GroupID != 77 {
				t.Fatalf("unexpected assign subscription payload: %+v", payload)
			}
			if payload.ValidityDays != 30 {
				t.Fatalf("expected subscription validity days 30, got %v", payload.ValidityDays)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":1,"user_id":32,"group_id":77,"status":"active"}}`))
		default:
			t.Fatalf("unexpected upstream path: %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	localUserID, _ := createUserViaAPI(t, mux, "assign-upstream-user@example.com", "Assign Upstream User", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "assign-upstream-admin@example.com", "Assign Upstream Admin", "admin", "test-admin-secret")
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, upstream_user_id, access_token, refresh_token) VALUES (?, ?, ?, ?);`, localUserID, 32, "upstream-user-token", "upstream-refresh-token"); err != nil {
		t.Fatalf("seed sub2api auth token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/users/assign-package", bytes.NewReader([]byte(`{"user_id":32,"tier_code":"pro-monthly","password":"LatestPassword#123"}`)))
	req.Header.Set("Content-Type", "application/json")
	setBearerAuth(req, adminSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if assignedUserID != 32 {
		t.Fatalf("expected upstream assign user id 32, got %d", assignedUserID)
	}
	if got := strings.Join(upstreamCalls, ","); got != "login,list-subscriptions,assign,list-key,key" {
		t.Fatalf("expected upstream call order login,list-subscriptions,assign,list-key,key; got %s", got)
	}
	var storedAccess string
	if err := database.QueryRowContext(ctx, `SELECT access_token FROM als_sub2api_auth_tokens WHERE user_id = ?;`, localUserID).Scan(&storedAccess); err != nil {
		t.Fatalf("query refreshed upstream token: %v", err)
	}
	if storedAccess != "new-upstream-user-token" {
		t.Fatalf("expected refreshed upstream token, got %q", storedAccess)
	}

	var subCount int64
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_subscriptions WHERE user_id = ? AND status = 'active' AND ended_at IS NULL;`, localUserID).Scan(&subCount); err != nil {
		t.Fatalf("count local subscriptions: %v", err)
	}
	if subCount != 1 {
		t.Fatalf("expected one local subscription for local user %d, got %d", localUserID, subCount)
	}
}

func TestAdminAssignPackageAcceptsEmail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "email-pro-monthly", "Email Pro Monthly")
	if _, err := database.ExecContext(ctx, `UPDATE als_tiers SET price_micros = 29900000, value_type = 'days', value_amount = 30, is_enabled = 1 WHERE id = ?;`, tierID); err != nil {
		t.Fatalf("update tier fields: %v", err)
	}
	insertTierGroupBinding(t, ctx, database, tierID, 77)

	var assignedUserID int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.URL.Path == "/api/v1/admin/groups/all":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[{"id":77,"name":"Pro Subscription","subscription_type":"monthly"}]}`))
		case req.URL.Path == "/api/v1/admin/users/32/subscriptions" && req.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[]}`))
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"data":[],"total":0,"page":1,"per_page":100}}`))
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodPost:
			var payload proxy.CreateUserAPIKeyRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode api key payload: %v", err)
			}
			if payload.Name != "auto-key" || payload.GroupID != 77 {
				t.Fatalf("unexpected api key payload: %+v", payload)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"id":91,"name":"auto-key","key":"sk-test-auto","group_id":77,"status":"active"}}`))
		case req.URL.Path == "/api/v1/admin/subscriptions/assign":
			var payload proxy.AssignAdminSubscriptionRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode assign subscription payload: %v", err)
			}
			assignedUserID = payload.UserID
			if payload.UserID != 32 || payload.GroupID != 77 {
				t.Fatalf("unexpected assign subscription payload: %+v", payload)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":1,"user_id":32,"group_id":77,"status":"active"}}`))
		default:
			t.Fatalf("unexpected upstream path: %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	localUserID, _ := createUserViaAPI(t, mux, "assign-email-user@example.com", "Assign Email User", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "assign-email-admin@example.com", "Assign Email Admin", "admin", "test-admin-secret")
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, upstream_user_id, access_token, refresh_token) VALUES (?, ?, ?, ?);`, localUserID, 32, "upstream-user-token", "upstream-refresh-token"); err != nil {
		t.Fatalf("seed sub2api auth token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/users/assign-package", bytes.NewReader([]byte(`{"email":"assign-email-user@example.com","tier_code":"email-pro-monthly"}`)))
	req.Header.Set("Content-Type", "application/json")
	setBearerAuth(req, adminSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if assignedUserID != 32 {
		t.Fatalf("expected upstream assign user id 32, got %d", assignedUserID)
	}

	var subCount int64
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_subscriptions WHERE user_id = ? AND status = 'active' AND ended_at IS NULL;`, localUserID).Scan(&subCount); err != nil {
		t.Fatalf("count local subscriptions: %v", err)
	}
	if subCount != 1 {
		t.Fatalf("expected one local subscription for local user %d, got %d", localUserID, subCount)
	}
}

func TestAdminAssignPackageImportsExistingUpstreamUserID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "imported-pro", "Imported Pro")
	if _, err := database.ExecContext(ctx, `UPDATE als_tiers SET price_micros = 19900000, value_type = 'days', value_amount = 30, is_enabled = 1 WHERE id = ?;`, tierID); err != nil {
		t.Fatalf("update tier fields: %v", err)
	}
	insertTierGroupBinding(t, ctx, database, tierID, 77)

	var (
		assignedUserID int64
		keyPayload     proxy.CreateUserAPIKeyRequest
		upstreamCalls  []string
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.URL.Path == "/api/v1/admin/users/40" && req.Method == http.MethodGet:
			upstreamCalls = append(upstreamCalls, "get-user")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":40,"email":"external-user@example.com","username":"External User","allowed_groups":[]}}`))
		case req.URL.Path == "/api/v1/admin/groups/all":
			upstreamCalls = append(upstreamCalls, "groups")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[{"id":77,"name":"Imported Subscription","subscription_type":"monthly"}]}`))
		case req.URL.Path == "/api/v1/admin/users/40/subscriptions" && req.Method == http.MethodGet:
			upstreamCalls = append(upstreamCalls, "list-subscriptions")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[]}`))
		case req.URL.Path == "/api/v1/admin/subscriptions/assign" && req.Method == http.MethodPost:
			upstreamCalls = append(upstreamCalls, "assign")
			var payload proxy.AssignAdminSubscriptionRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode assign subscription payload: %v", err)
			}
			assignedUserID = payload.UserID
			if payload.UserID != 40 || payload.GroupID != 77 || payload.ValidityDays != 30 {
				t.Fatalf("unexpected assign subscription payload: %+v", payload)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":1,"user_id":40,"group_id":77,"status":"active"}}`))
		case req.URL.Path == "/api/v1/admin/users/40/api-keys" && req.Method == http.MethodGet:
			upstreamCalls = append(upstreamCalls, "admin-list-key")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"data":[],"total":0,"page":1,"per_page":100}}`))
		case req.URL.Path == "/api/v1/admin/users/40/api-keys" && req.Method == http.MethodPost:
			upstreamCalls = append(upstreamCalls, "admin-key")
			if err := json.NewDecoder(req.Body).Decode(&keyPayload); err != nil {
				t.Fatalf("decode api key payload: %v", err)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"id":91,"name":"auto-key","key":"sk-test-auto","group_id":77,"user_id":40,"status":"active"}}`))
		default:
			t.Fatalf("unexpected upstream path: %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	_, adminSessionToken := createUserViaAPI(t, mux, "assign-import-admin@example.com", "Assign Import Admin", "admin", "test-admin-secret")

	req := httptest.NewRequest(http.MethodPost, "/admin/users/assign-package", bytes.NewReader([]byte(`{"user_id":40,"tier_code":"imported-pro"}`)))
	req.Header.Set("Content-Type", "application/json")
	setBearerAuth(req, adminSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if assignedUserID != 40 {
		t.Fatalf("expected upstream assign user id 40, got %d", assignedUserID)
	}
	if keyPayload.Name != "auto-key" || keyPayload.GroupID != 77 {
		t.Fatalf("expected admin-created auto key in group 77, got %+v", keyPayload)
	}
	if got := strings.Join(upstreamCalls, ","); got != "get-user,groups,groups,list-subscriptions,assign,admin-list-key,admin-key" {
		t.Fatalf("unexpected upstream call order: %s", got)
	}

	var (
		localUserID      int64
		importedEmail    string
		upstreamUserID   int64
		storedAccess     string
		localSubCount    int64
		localWalletCount int64
	)
	if err := database.QueryRowContext(ctx, `
		SELECT u.id, u.email, tok.upstream_user_id, tok.access_token
		FROM als_users u
		JOIN als_sub2api_auth_tokens tok ON tok.user_id = u.id
		WHERE u.email = 'external-user@example.com';
	`).Scan(&localUserID, &importedEmail, &upstreamUserID, &storedAccess); err != nil {
		t.Fatalf("query imported local user: %v", err)
	}
	if importedEmail != "external-user@example.com" || upstreamUserID != 40 {
		t.Fatalf("unexpected imported mapping local=%d email=%q upstream=%d", localUserID, importedEmail, upstreamUserID)
	}
	if storedAccess != "" {
		t.Fatalf("expected imported user to have no user access token, got %q", storedAccess)
	}
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_subscriptions WHERE user_id = ? AND status = 'active' AND ended_at IS NULL;`, localUserID).Scan(&localSubCount); err != nil {
		t.Fatalf("count local subscriptions: %v", err)
	}
	if localSubCount != 1 {
		t.Fatalf("expected one local subscription for imported user, got %d", localSubCount)
	}
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_user_wallets WHERE user_id = ?;`, localUserID).Scan(&localWalletCount); err != nil {
		t.Fatalf("count local wallet rows: %v", err)
	}
	if localWalletCount != 1 {
		t.Fatalf("expected wallet for imported user, got %d", localWalletCount)
	}
}

func TestAdminAssignPackageReturnsErrorWhenFulfillmentFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "daily", "Daily")
	if _, err := database.ExecContext(ctx, `UPDATE als_tiers SET price_micros = 1000000, value_type = 'days', value_amount = 1, is_enabled = 1 WHERE id = ?;`, tierID); err != nil {
		t.Fatalf("update tier fields: %v", err)
	}
	insertTierGroupBinding(t, ctx, database, tierID, 77)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/admin/groups/all":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[{"id":77,"name":"Daily Subscription","subscription_type":"daily"}]}`))
		case "/api/v1/admin/users/34/subscriptions":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[]}`))
		case "/api/v1/admin/subscriptions/assign":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"code":400,"message":"group is not a subscription type","reason":"GROUP_NOT_SUBSCRIPTION_TYPE"}`))
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
	localUserID, _ := createUserViaAPI(t, mux, "assign-fail-user@example.com", "Assign Fail User", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "assign-fail-admin@example.com", "Assign Fail Admin", "admin", "test-admin-secret")
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, upstream_user_id, access_token, refresh_token) VALUES (?, ?, ?, ?);`, localUserID, 34, "upstream-user-token", "upstream-refresh-token"); err != nil {
		t.Fatalf("seed sub2api auth token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/users/assign-package", bytes.NewReader([]byte(`{"user_id":34,"tier_code":"daily"}`)))
	req.Header.Set("Content-Type", "application/json")
	setBearerAuth(req, adminSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	var response adminAssignPackageResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode assign package response: %v", err)
	}
	if response.FulfillmentJob == nil || response.FulfillmentJob.Status != fulfillment.StatusFailedTerminal {
		t.Fatalf("expected failed terminal fulfillment job, got %+v", response.FulfillmentJob)
	}
	if response.FulfillmentJob.ErrorMessage == nil || !strings.Contains(*response.FulfillmentJob.ErrorMessage, "GROUP_NOT_SUBSCRIPTION_TYPE") {
		t.Fatalf("expected upstream error in fulfillment response, got %+v", response.FulfillmentJob)
	}
}

func TestAdminAssignPackageGrantsStandardGroupAndCreatesKey(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "standard-group-package", "Standard Group Package")
	if _, err := database.ExecContext(ctx, `UPDATE als_tiers SET price_micros = 1000000, value_type = '', value_amount = 0, is_enabled = 1 WHERE id = ?;`, tierID); err != nil {
		t.Fatalf("update tier fields: %v", err)
	}
	insertTierGroupBinding(t, ctx, database, tierID, 88)

	var grantPayload struct {
		AllowedGroups []int64 `json:"allowed_groups"`
	}
	var keyPayload proxy.CreateUserAPIKeyRequest
	var upstreamCalls []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.URL.Path == "/api/v1/admin/groups/all":
			upstreamCalls = append(upstreamCalls, "groups")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[{"id":88,"name":"Plain Group","subscription_type":"standard"}]}`))
		case req.URL.Path == "/api/v1/admin/users/88" && req.Method == http.MethodGet:
			upstreamCalls = append(upstreamCalls, "get-user")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":88,"email":"standard-user@example.com","allowed_groups":[11]}}`))
		case req.URL.Path == "/api/v1/admin/users/88" && req.Method == http.MethodPut:
			upstreamCalls = append(upstreamCalls, "grant-group")
			if err := json.NewDecoder(req.Body).Decode(&grantPayload); err != nil {
				t.Fatalf("decode grant group payload: %v", err)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":88,"email":"standard-user@example.com","allowed_groups":[11,88]}}`))
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodGet:
			upstreamCalls = append(upstreamCalls, "list-key")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"data":[],"total":0,"page":1,"per_page":100}}`))
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodPost:
			upstreamCalls = append(upstreamCalls, "key")
			if err := json.NewDecoder(req.Body).Decode(&keyPayload); err != nil {
				t.Fatalf("decode api key payload: %v", err)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"id":91,"name":"auto-key","key":"sk-test-auto","group_id":88,"status":"active"}}`))
		default:
			t.Fatalf("unexpected upstream path: %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	localUserID, _ := createUserViaAPI(t, mux, "assign-invalid-group-user@example.com", "Assign Invalid Group User", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "assign-invalid-group-admin@example.com", "Assign Invalid Group Admin", "admin", "test-admin-secret")
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, upstream_user_id, access_token, refresh_token) VALUES (?, ?, ?, ?);`, localUserID, 88, "upstream-user-token", "upstream-refresh-token"); err != nil {
		t.Fatalf("seed sub2api auth token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/users/assign-package", bytes.NewReader([]byte(`{"user_id":88,"tier_code":"standard-group-package"}`)))
	req.Header.Set("Content-Type", "application/json")
	setBearerAuth(req, adminSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if !reflect.DeepEqual(grantPayload.AllowedGroups, []int64{11, 88}) {
		t.Fatalf("expected merged allowed groups [11 88], got %+v", grantPayload.AllowedGroups)
	}
	if keyPayload.GroupID != 88 {
		t.Fatalf("expected auto key in group 88, got %+v", keyPayload)
	}
	if got := strings.Join(upstreamCalls, ","); got != "groups,groups,get-user,grant-group,list-key,key" {
		t.Fatalf("expected upstream call order groups,groups,get-user,grant-group,list-key,key; got %s", got)
	}

	var subCount int64
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_subscriptions WHERE user_id = ? AND status = 'active' AND ended_at IS NULL;`, localUserID).Scan(&subCount); err != nil {
		t.Fatalf("count local subscriptions: %v", err)
	}
	if subCount != 1 {
		t.Fatalf("expected local active subscription record, got %d", subCount)
	}
}

func TestPackagePurchaseRenewsExistingSubscriptionAndSkipsExistingAutoKey(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "pro-monthly", "Pro Monthly")
	if _, err := database.ExecContext(ctx, `UPDATE als_tiers SET price_micros = 29900000, value_type = 'days', value_amount = 30, is_enabled = 1, is_published = 1 WHERE id = ?;`, tierID); err != nil {
		t.Fatalf("update tier fields: %v", err)
	}
	insertTierGroupBinding(t, ctx, database, tierID, 77)

	var (
		upstreamCalls []string
		extendPayload proxy.ExtendAdminSubscriptionRequest
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.URL.Path == "/api/v1/admin/groups/all":
			upstreamCalls = append(upstreamCalls, "groups")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[{"id":77,"name":"Pro Subscription","subscription_type":"monthly"}]}`))
		case req.URL.Path == "/api/v1/admin/users/501/subscriptions" && req.Method == http.MethodGet:
			upstreamCalls = append(upstreamCalls, "list-subscriptions")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[{"id":700,"user_id":501,"group_id":77,"status":"active"}]}`))
		case req.URL.Path == "/api/v1/admin/subscriptions/700/extend" && req.Method == http.MethodPost:
			upstreamCalls = append(upstreamCalls, "extend")
			if got := req.Header.Get("Idempotency-Key"); got != "idem-renew-package:extend-subscription:77" {
				t.Fatalf("unexpected extend idempotency key: %q", got)
			}
			if err := json.NewDecoder(req.Body).Decode(&extendPayload); err != nil {
				t.Fatalf("decode extend payload: %v", err)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":700,"user_id":501,"group_id":77,"status":"active"}}`))
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodGet:
			upstreamCalls = append(upstreamCalls, "list-key")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"data":[{"id":91,"name":"auto-key","group_id":77,"status":"active"}],"total":1,"page":1,"per_page":100}}`))
		case req.URL.Path == "/api/v1/admin/subscriptions/assign":
			t.Fatalf("repeat purchase should extend existing subscription, not assign a new one")
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodPost:
			t.Fatalf("existing auto-key should not be created again")
		default:
			t.Fatalf("unexpected upstream path: %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	localUserID, _ := createUserViaAPI(t, mux, "renew-package-user@example.com", "Renew Package User", "user", "")
	_, adminSessionToken := createUserViaAPI(t, mux, "renew-package-admin@example.com", "Renew Package Admin", "admin", "test-admin-secret")
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, upstream_user_id, access_token, refresh_token) VALUES (?, ?, ?, ?);`, localUserID, 501, "upstream-user-token", "upstream-refresh-token"); err != nil {
		t.Fatalf("seed sub2api auth token: %v", err)
	}
	subscriptionID := insertActiveSubscription(t, ctx, database, localUserID, tierID, "2099-01-01T00:00:00Z")
	if _, err := database.ExecContext(ctx, `UPDATE als_subscriptions SET expires_at = ? WHERE id = ?;`, "2099-01-31T00:00:00Z", subscriptionID); err != nil {
		t.Fatalf("seed subscription expiry: %v", err)
	}

	body := `{"payment_event_id":"evt_renew_package","user_id":` + strconv.FormatInt(localUserID, 10) + `,"tier_code":"pro-monthly","order_id":"cs_renew_package"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/fulfillment/payment-success", bytes.NewReader([]byte(body)))
	req.Header.Set("Idempotency-Key", "idem-renew-package")
	setBearerAuth(req, adminSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusAccepted, rec.Code, rec.Body.String())
	}
	if got := strings.Join(upstreamCalls, ","); got != "groups,list-subscriptions,extend,list-key" {
		t.Fatalf("expected renewal upstream calls groups,list-subscriptions,extend,list-key; got %s", got)
	}
	if extendPayload.Days != 30 {
		t.Fatalf("expected upstream extend days 30, got %+v", extendPayload)
	}

	var subCount int64
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_subscriptions WHERE user_id = ? AND status = 'active' AND ended_at IS NULL;`, localUserID).Scan(&subCount); err != nil {
		t.Fatalf("count local subscriptions: %v", err)
	}
	if subCount != 1 {
		t.Fatalf("expected existing local subscription to be reused, got %d active records", subCount)
	}

	var expiresAt string
	if err := database.QueryRowContext(ctx, `SELECT expires_at FROM als_subscriptions WHERE id = ?;`, subscriptionID).Scan(&expiresAt); err != nil {
		t.Fatalf("query local subscription expiry: %v", err)
	}
	if expiresAt != "2099-03-02T00:00:00Z" {
		t.Fatalf("expected local expiry extended to 2099-03-02T00:00:00Z, got %q", expiresAt)
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

	deleteMissingReq := httptest.NewRequest(http.MethodDelete, "/admin/packages/missing", nil)
	setBearerAuth(deleteMissingReq, adminSessionToken)
	deleteMissingRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteMissingRec, deleteMissingReq)
	if deleteMissingRec.Code != http.StatusNotFound {
		t.Fatalf("expected missing delete status %d, got %d", http.StatusNotFound, deleteMissingRec.Code)
	}
}

func TestAdminDeletePackageRemovesLocalTierAndBindings(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "delete-me", "Delete Me")
	insertTierGroupBinding(t, ctx, database, tierID, 11)

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})
	_, adminSessionToken := createUserViaAPI(t, mux, "package-delete-admin@example.com", "Package Delete Admin", "admin", "test-admin-secret")

	deleteReq := httptest.NewRequest(http.MethodDelete, "/admin/packages/delete-me", nil)
	setBearerAuth(deleteReq, adminSessionToken)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete status %d, got %d body=%s", http.StatusOK, deleteRec.Code, deleteRec.Body.String())
	}

	var tierCount int64
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_tiers WHERE code = ?;`, "delete-me").Scan(&tierCount); err != nil {
		t.Fatalf("count deleted tier: %v", err)
	}
	if tierCount != 0 {
		t.Fatalf("expected deleted tier count 0, got %d", tierCount)
	}

	var bindingCount int64
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_tier_group_bindings WHERE tier_id = ?;`, tierID).Scan(&bindingCount); err != nil {
		t.Fatalf("count deleted tier bindings: %v", err)
	}
	if bindingCount != 0 {
		t.Fatalf("expected deleted tier binding count 0, got %d", bindingCount)
	}
}

func TestAdminPackageCRUDDoesNotTouchSub2API(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)

	upstreamCallCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		upstreamCallCount++
		t.Fatalf("package CRUD should not call upstream, got %s %s", req.Method, req.URL.Path)
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
	_, adminSessionToken := createUserViaAPI(t, mux, "package-subscription-admin@example.com", "Package Subscription Admin", "admin", "test-admin-secret")

	createReq := httptest.NewRequest(http.MethodPost, "/admin/packages", bytes.NewReader([]byte(`{"code":"local-only","name":"Local Only","group_ids":[11],"is_visible":true,"is_published":false}`)))
	setBearerAuth(createReq, adminSessionToken)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected local create status %d, got %d body=%s", http.StatusCreated, createRec.Code, createRec.Body.String())
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/admin/packages/local-only", bytes.NewReader([]byte(`{"name":"Local Only Updated","group_ids":[11,22],"value_type":"days","value_amount":30,"is_visible":false,"is_published":true}`)))
	setBearerAuth(updateReq, adminSessionToken)
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected local update status %d, got %d body=%s", http.StatusOK, updateRec.Code, updateRec.Body.String())
	}

	if upstreamCallCount != 0 {
		t.Fatalf("expected no upstream calls, got %d", upstreamCallCount)
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
	adminID, adminSessionToken := createUserViaAPI(t, mux, "groups-admin@example.com", "Groups Admin", "admin", "test-admin-secret")
	insertActiveSubscription(t, ctx, database, userID, tierID, "2026-01-01T00:00:00Z")
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, access_token, refresh_token) VALUES (?, ?, ?);`, userID, "upstream-user-token", "upstream-user-refresh-token"); err != nil {
		t.Fatalf("seed user sub2api auth token: %v", err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, access_token, refresh_token) VALUES (?, ?, ?);`, adminID, "upstream-admin-token", "upstream-admin-refresh-token"); err != nil {
		t.Fatalf("seed admin sub2api auth token: %v", err)
	}

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
	type upstreamCall struct {
		Method string
		Path   string
		Auth   string
	}
	var upstreamCalls []upstreamCall

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		upstreamCalls = append(upstreamCalls, upstreamCall{
			Method: req.Method,
			Path:   req.URL.Path,
			Auth:   req.Header.Get("Authorization"),
		})
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"data":[{"id":11,"name":"Starter Group","code":"starter-group","platform":"openai","status":"active"},{"id":22,"name":"Pro Group","code":"pro-group","platform":"openai","status":"active"}]}`))
		case "/api/v1/api-keys":
			_, _ = w.Write([]byte(`{"data":{"data":[{"id":101,"name":"starter-key","key":"sk-starter","group_id":11,"group":{"id":11,"name":"Starter Group"},"status":"active","quota":0,"quota_used":0,"expires_at":"","created_at":"2026-01-01T00:00:00Z"},{"id":202,"name":"pro-key","key":"sk-pro","group_id":22,"group":{"id":22,"name":"Pro Group"},"status":"active","quota":0,"quota_used":0,"expires_at":"","created_at":"2026-01-01T00:00:00Z"}],"total":2,"page":1,"per_page":20}}`))
		case "/api/v1/api-keys/101":
			if req.Method == http.MethodDelete {
				_, _ = w.Write([]byte(`{"data":{"message":"API key deleted successfully"}}`))
				return
			}
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
	adminID, adminSessionToken := createUserViaAPI(t, mux, "keys-admin@example.com", "Keys Admin", "admin", "test-admin-secret")
	insertActiveSubscription(t, ctx, database, userID, tierID, "2026-01-01T00:00:00Z")
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, access_token, refresh_token) VALUES (?, ?, ?);`, userID, "upstream-user-token", "upstream-user-refresh-token"); err != nil {
		t.Fatalf("seed user sub2api auth token: %v", err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, access_token, refresh_token) VALUES (?, ?, ?);`, adminID, "upstream-admin-token", "upstream-admin-refresh-token"); err != nil {
		t.Fatalf("seed admin sub2api auth token: %v", err)
	}

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
	if len(upstreamCalls) != 2 {
		t.Fatalf("expected 2 upstream calls after list, got %d", len(upstreamCalls))
	}
	if upstreamCalls[0].Path != "/api/v1/groups/available" || upstreamCalls[0].Auth != "Bearer upstream-user-token" {
		t.Fatalf("unexpected list groups upstream call: %+v", upstreamCalls[0])
	}
	if upstreamCalls[1].Path != "/api/v1/api-keys" || upstreamCalls[1].Auth != "Bearer upstream-user-token" {
		t.Fatalf("unexpected list api keys upstream call: %+v", upstreamCalls[1])
	}

	allowedDetailReq := httptest.NewRequest(http.MethodGet, "/api-keys/101", nil)
	setBearerAuth(allowedDetailReq, userSessionToken)
	allowedDetailRec := httptest.NewRecorder()
	mux.ServeHTTP(allowedDetailRec, allowedDetailReq)
	if allowedDetailRec.Code != http.StatusOK {
		t.Fatalf("expected allowed detail status %d, got %d body=%s", http.StatusOK, allowedDetailRec.Code, allowedDetailRec.Body.String())
	}
	if len(upstreamCalls) != 4 {
		t.Fatalf("expected 4 upstream calls after allowed detail, got %d", len(upstreamCalls))
	}
	if upstreamCalls[2].Path != "/api/v1/groups/available" || upstreamCalls[2].Auth != "Bearer upstream-user-token" {
		t.Fatalf("unexpected allowed detail groups upstream call: %+v", upstreamCalls[2])
	}
	if upstreamCalls[3].Path != "/api/v1/api-keys/101" || upstreamCalls[3].Auth != "Bearer upstream-user-token" {
		t.Fatalf("unexpected allowed detail upstream call: %+v", upstreamCalls[3])
	}

	blockedDetailReq := httptest.NewRequest(http.MethodGet, "/api-keys/202", nil)
	setBearerAuth(blockedDetailReq, userSessionToken)
	blockedDetailRec := httptest.NewRecorder()
	mux.ServeHTTP(blockedDetailRec, blockedDetailReq)
	if blockedDetailRec.Code != http.StatusForbidden {
		t.Fatalf("expected blocked detail status %d, got %d body=%s", http.StatusForbidden, blockedDetailRec.Code, blockedDetailRec.Body.String())
	}
	if len(upstreamCalls) != 6 {
		t.Fatalf("expected 6 upstream calls after blocked detail, got %d", len(upstreamCalls))
	}
	if upstreamCalls[4].Path != "/api/v1/groups/available" || upstreamCalls[4].Auth != "Bearer upstream-user-token" {
		t.Fatalf("unexpected blocked detail groups upstream call: %+v", upstreamCalls[4])
	}
	if upstreamCalls[5].Path != "/api/v1/api-keys/202" || upstreamCalls[5].Auth != "Bearer upstream-user-token" {
		t.Fatalf("unexpected blocked detail upstream call: %+v", upstreamCalls[5])
	}

	adminDetailReq := httptest.NewRequest(http.MethodGet, "/api-keys/202", nil)
	setBearerAuth(adminDetailReq, adminSessionToken)
	adminDetailRec := httptest.NewRecorder()
	mux.ServeHTTP(adminDetailRec, adminDetailReq)
	if adminDetailRec.Code != http.StatusOK {
		t.Fatalf("expected admin detail status %d, got %d body=%s", http.StatusOK, adminDetailRec.Code, adminDetailRec.Body.String())
	}
	if len(upstreamCalls) != 7 {
		t.Fatalf("expected 7 upstream calls after admin detail, got %d", len(upstreamCalls))
	}
	if upstreamCalls[6].Path != "/api/v1/api-keys/202" || upstreamCalls[6].Auth != "Bearer upstream-admin-token" {
		t.Fatalf("unexpected admin detail upstream call: %+v", upstreamCalls[6])
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api-keys/101", nil)
	setBearerAuth(deleteReq, userSessionToken)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete status %d, got %d body=%s", http.StatusOK, deleteRec.Code, deleteRec.Body.String())
	}
	if len(upstreamCalls) != 10 {
		t.Fatalf("expected 10 upstream calls after delete, got %d", len(upstreamCalls))
	}
	if upstreamCalls[7].Method != http.MethodGet || upstreamCalls[7].Path != "/api/v1/groups/available" || upstreamCalls[7].Auth != "Bearer upstream-user-token" {
		t.Fatalf("unexpected delete groups upstream call: %+v", upstreamCalls[7])
	}
	if upstreamCalls[8].Method != http.MethodGet || upstreamCalls[8].Path != "/api/v1/api-keys/101" || upstreamCalls[8].Auth != "Bearer upstream-user-token" {
		t.Fatalf("unexpected delete preflight upstream call: %+v", upstreamCalls[8])
	}
	if upstreamCalls[9].Method != http.MethodDelete || upstreamCalls[9].Path != "/api/v1/api-keys/101" || upstreamCalls[9].Auth != "Bearer upstream-user-token" {
		t.Fatalf("unexpected delete upstream call: %+v", upstreamCalls[9])
	}
}

func TestAPIKeyDeleteRejectsAutoKeyForUsersAndAdmins(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "starter", "Starter")
	insertTierGroupBinding(t, ctx, database, tierID, 11)
	type upstreamCall struct {
		Method string
		Path   string
		Auth   string
	}
	var upstreamCalls []upstreamCall

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		upstreamCalls = append(upstreamCalls, upstreamCall{
			Method: req.Method,
			Path:   req.URL.Path,
			Auth:   req.Header.Get("Authorization"),
		})
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"data":[{"id":11,"name":"Starter Group","platform":"openai","status":"active"}]}`))
		case "/api/v1/api-keys/303":
			if req.Method == http.MethodDelete {
				t.Fatalf("protected auto-key should not be deleted upstream")
			}
			_, _ = w.Write([]byte(`{"data":{"id":303,"name":"auto-key","key":"sk-auto","group_id":11,"group":{"id":11,"name":"Starter Group"},"status":"active"}}`))
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
	userID, userSessionToken := createUserViaAPI(t, mux, "auto-key-user@example.com", "Auto Key User", "user", "")
	adminID, adminSessionToken := createUserViaAPI(t, mux, "auto-key-admin@example.com", "Auto Key Admin", "admin", "test-admin-secret")
	insertActiveSubscription(t, ctx, database, userID, tierID, "2026-01-01T00:00:00Z")
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, access_token, refresh_token) VALUES (?, ?, ?);`, userID, "upstream-user-token", "upstream-user-refresh-token"); err != nil {
		t.Fatalf("seed user sub2api auth token: %v", err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, access_token, refresh_token) VALUES (?, ?, ?);`, adminID, "upstream-admin-token", "upstream-admin-refresh-token"); err != nil {
		t.Fatalf("seed admin sub2api auth token: %v", err)
	}

	userDeleteReq := httptest.NewRequest(http.MethodDelete, "/api-keys/303", nil)
	setBearerAuth(userDeleteReq, userSessionToken)
	userDeleteRec := httptest.NewRecorder()
	mux.ServeHTTP(userDeleteRec, userDeleteReq)
	if userDeleteRec.Code != http.StatusForbidden {
		t.Fatalf("expected user auto-key delete status %d, got %d body=%s", http.StatusForbidden, userDeleteRec.Code, userDeleteRec.Body.String())
	}

	adminDeleteReq := httptest.NewRequest(http.MethodDelete, "/api-keys/303", nil)
	setBearerAuth(adminDeleteReq, adminSessionToken)
	adminDeleteRec := httptest.NewRecorder()
	mux.ServeHTTP(adminDeleteRec, adminDeleteReq)
	if adminDeleteRec.Code != http.StatusForbidden {
		t.Fatalf("expected admin auto-key delete status %d, got %d body=%s", http.StatusForbidden, adminDeleteRec.Code, adminDeleteRec.Body.String())
	}

	for _, call := range upstreamCalls {
		if call.Method == http.MethodDelete {
			t.Fatalf("protected auto-key delete reached upstream: %+v", call)
		}
	}
}

func TestSubscriptionsSummaryReturnsLocalPackageConfiguration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "pro-monthly", "Pro Monthly")
	if _, err := database.ExecContext(ctx, `
		UPDATE als_tiers
		SET price_micros = 29900000,
			value_type = 'days',
			value_amount = 30,
			description = 'Configured package description.',
			features_json = '["Feature A","Feature B"]',
			is_enabled = 1,
			is_visible = 1,
			is_published = 1
		WHERE id = ?;
	`, tierID); err != nil {
		t.Fatalf("update tier fields: %v", err)
	}
	insertTierGroupBinding(t, ctx, database, tierID, 11)
	insertTierGroupBinding(t, ctx, database, tierID, 22)

	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		upstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"active_count":1,"subscriptions":[{"id":9001,"group_id":11,"group_name":"Upstream Group","status":"active"}]}}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithHTTPClient(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{ProxyClient: proxyClient})
	userID, sessionToken := createUserViaAPI(t, mux, "summary-package-user@example.com", "Summary Package User", "user", "")
	insertActiveSubscription(t, ctx, database, userID, tierID, "2026-01-01 00:00:00")

	req := httptest.NewRequest(http.MethodGet, "/subscriptions/summary", nil)
	setBearerAuth(req, sessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if upstreamCalls != 0 {
		t.Fatalf("expected local package summary to avoid upstream calls, got %d", upstreamCalls)
	}

	var payload struct {
		Data struct {
			ActiveCount   int `json:"active_count"`
			Subscriptions []struct {
				GroupID     string   `json:"group_id"`
				GroupName   string   `json:"group_name"`
				PackageCode string   `json:"package_code"`
				PackageName string   `json:"package_name"`
				GroupIDs    []int64  `json:"group_ids"`
				ValueType   string   `json:"value_type"`
				ValueAmount int64    `json:"value_amount"`
				ExpiresAt   string   `json:"expires_at"`
				Features    []string `json:"features"`
				Source      string   `json:"source"`
			} `json:"subscriptions"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode subscriptions summary: %v", err)
	}
	if payload.Data.ActiveCount != 1 || len(payload.Data.Subscriptions) != 1 {
		t.Fatalf("unexpected subscription count payload: %+v", payload.Data)
	}
	sub := payload.Data.Subscriptions[0]
	if sub.GroupID != "pro-monthly" || sub.GroupName != "Pro Monthly" || sub.PackageCode != "pro-monthly" || sub.PackageName != "Pro Monthly" {
		t.Fatalf("expected package identity, got %+v", sub)
	}
	if !reflect.DeepEqual(sub.GroupIDs, []int64{11, 22}) {
		t.Fatalf("expected package group ids [11 22], got %+v", sub.GroupIDs)
	}
	if sub.ValueType != "days" || sub.ValueAmount != 30 {
		t.Fatalf("expected package duration days/30, got %s/%d", sub.ValueType, sub.ValueAmount)
	}
	if sub.ExpiresAt != "2026-01-31T00:00:00Z" {
		t.Fatalf("expected expires_at 2026-01-31T00:00:00Z, got %q", sub.ExpiresAt)
	}
	if !reflect.DeepEqual(sub.Features, []string{"Feature A", "Feature B"}) {
		t.Fatalf("expected package features, got %+v", sub.Features)
	}
	if sub.Source != "package" {
		t.Fatalf("expected source package, got %q", sub.Source)
	}
}

func TestDistributorCanListBoundUserStatsAndAssignPackage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "distributor-package", "Distributor Package")
	if _, err := database.ExecContext(ctx, `UPDATE als_tiers SET level = 'distributor', price_micros = 1000000, value_type = '', value_amount = 0, is_enabled = 1, is_published = 1 WHERE id = ?;`, tierID); err != nil {
		t.Fatalf("update tier fields: %v", err)
	}
	insertTierGroupBinding(t, ctx, database, tierID, 88)

	var grantPayload struct {
		AllowedGroups []int64 `json:"allowed_groups"`
	}
	var keyPayload proxy.CreateUserAPIKeyRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.URL.Path == "/api/v1/admin/groups/all":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[{"id":88,"name":"Standard Group","subscription_type":""}]}`))
		case req.URL.Path == "/api/v1/admin/users/50" && req.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":50,"allowed_groups":[11]}}`))
		case req.URL.Path == "/api/v1/admin/users/50" && req.Method == http.MethodPut:
			if err := json.NewDecoder(req.Body).Decode(&grantPayload); err != nil {
				t.Fatalf("decode grant payload: %v", err)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":50,"allowed_groups":[11,88]}}`))
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"data":[],"total":0,"page":1,"per_page":100}}`))
		case req.URL.Path == "/api/v1/api-keys" && req.Method == http.MethodPost:
			if err := json.NewDecoder(req.Body).Decode(&keyPayload); err != nil {
				t.Fatalf("decode api key payload: %v", err)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"id":91,"name":"auto-key","key":"sk-test-auto","group_id":88,"status":"active"}}`))
		default:
			t.Fatalf("unexpected upstream path: %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", ProxyClient: proxyClient})
	_, adminSessionToken := createUserViaAPI(t, mux, "distributor-admin@example.com", "Distributor Admin", "admin", "test-admin-secret")
	distributorID, distributorSessionToken := createUserViaAPI(t, mux, "distributor@example.com", "Distributor", "user", "")
	if _, err := database.ExecContext(ctx, `UPDATE als_users SET role = 'distributor' WHERE id = ?;`, distributorID); err != nil {
		t.Fatalf("promote distributor role: %v", err)
	}
	targetUserID, _ := createUserViaAPI(t, mux, "distributor-user@example.com", "Distributor User", "user", "")
	otherUserID, _ := createUserViaAPI(t, mux, "other-distributor-user@example.com", "Other User", "user", "")
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, upstream_user_id, access_token, refresh_token) VALUES (?, ?, ?, ?);`, targetUserID, 50, "upstream-user-token", "upstream-refresh-token"); err != nil {
		t.Fatalf("seed sub2api auth token: %v", err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO als_sub2api_auth_tokens(user_id, upstream_user_id, access_token, refresh_token) VALUES (?, ?, ?, ?);`, otherUserID, 60, "other-upstream-token", "other-upstream-refresh"); err != nil {
		t.Fatalf("seed other sub2api auth token: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO als_user_usage_daily(user_id, usage_date, request_count, input_tokens, output_tokens, total_tokens, actual_cost_micros)
		VALUES (?, '2026-06-01', 3, 100, 200, 300, 1230000);
	`, targetUserID); err != nil {
		t.Fatalf("seed daily usage: %v", err)
	}

	bindReq := httptest.NewRequest(http.MethodPost, "/admin/distributor/users", bytes.NewReader([]byte(`{"distributor_email":"distributor@example.com","email":"distributor-user@example.com"}`)))
	bindReq.Header.Set("Content-Type", "application/json")
	setBearerAuth(bindReq, adminSessionToken)
	bindRec := httptest.NewRecorder()
	mux.ServeHTTP(bindRec, bindReq)
	if bindRec.Code != http.StatusOK {
		t.Fatalf("expected bind status %d, got %d body=%s", http.StatusOK, bindRec.Code, bindRec.Body.String())
	}

	adminInvitationsReq := httptest.NewRequest(http.MethodGet, "/admin/distributor/users", nil)
	setBearerAuth(adminInvitationsReq, adminSessionToken)
	adminInvitationsRec := httptest.NewRecorder()
	mux.ServeHTTP(adminInvitationsRec, adminInvitationsReq)
	if adminInvitationsRec.Code != http.StatusOK {
		t.Fatalf("expected admin invitations status %d, got %d body=%s", http.StatusOK, adminInvitationsRec.Code, adminInvitationsRec.Body.String())
	}
	var adminInvitationsPayload listDistributorInvitationsResponse
	if err := json.NewDecoder(adminInvitationsRec.Body).Decode(&adminInvitationsPayload); err != nil {
		t.Fatalf("decode admin distributor invitations: %v", err)
	}
	if len(adminInvitationsPayload.Invitations) != 1 || adminInvitationsPayload.Invitations[0].DistributorUserID != distributorID || adminInvitationsPayload.Invitations[0].UserID != targetUserID {
		t.Fatalf("unexpected admin distributor invitations: %+v", adminInvitationsPayload.Invitations)
	}

	invitationsReq := httptest.NewRequest(http.MethodGet, "/distributor/invitations", nil)
	setBearerAuth(invitationsReq, distributorSessionToken)
	invitationsRec := httptest.NewRecorder()
	mux.ServeHTTP(invitationsRec, invitationsReq)
	if invitationsRec.Code != http.StatusOK {
		t.Fatalf("expected distributor invitations status %d, got %d body=%s", http.StatusOK, invitationsRec.Code, invitationsRec.Body.String())
	}
	var invitationsPayload listDistributorInvitationsResponse
	if err := json.NewDecoder(invitationsRec.Body).Decode(&invitationsPayload); err != nil {
		t.Fatalf("decode distributor invitations: %v", err)
	}
	if len(invitationsPayload.Invitations) != 1 || invitationsPayload.Invitations[0].UserID != targetUserID {
		t.Fatalf("unexpected distributor invitations: %+v", invitationsPayload.Invitations)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/distributor/users", nil)
	setBearerAuth(listReq, distributorSessionToken)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d body=%s", http.StatusOK, listRec.Code, listRec.Body.String())
	}
	var listPayload listDistributorUsersResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode distributor users: %v", err)
	}
	if len(listPayload.Users) != 1 || listPayload.Users[0].UserID != targetUserID {
		t.Fatalf("unexpected distributor users: %+v", listPayload.Users)
	}
	if listPayload.Users[0].TotalTokens != 300 || listPayload.Users[0].ActiveDays != 1 || listPayload.Users[0].ActualCostMicros != 1230000 {
		t.Fatalf("unexpected distributor usage stats: %+v", listPayload.Users[0])
	}

	assignReq := httptest.NewRequest(http.MethodPost, "/distributor/assign-package", bytes.NewReader([]byte(`{"email":"distributor-user@example.com","tier_code":"distributor-package"}`)))
	assignReq.Header.Set("Content-Type", "application/json")
	setBearerAuth(assignReq, distributorSessionToken)
	assignRec := httptest.NewRecorder()
	mux.ServeHTTP(assignRec, assignReq)
	if assignRec.Code != http.StatusOK {
		t.Fatalf("expected distributor assign status %d, got %d body=%s", http.StatusOK, assignRec.Code, assignRec.Body.String())
	}
	if !reflect.DeepEqual(grantPayload.AllowedGroups, []int64{11, 88}) {
		t.Fatalf("expected merged allowed groups [11 88], got %+v", grantPayload.AllowedGroups)
	}
	if keyPayload.Name != "auto-key" || keyPayload.GroupID != 88 {
		t.Fatalf("expected distributor auto key in group 88, got %+v", keyPayload)
	}
	var assignmentCount int64
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM als_distributor_package_assignments WHERE distributor_user_id = ? AND target_user_id = ?;`, distributorID, targetUserID).Scan(&assignmentCount); err != nil {
		t.Fatalf("count distributor assignments: %v", err)
	}
	if assignmentCount != 1 {
		t.Fatalf("expected one distributor assignment, got %d", assignmentCount)
	}
	var assignmentPriceMicros int64
	if err := database.QueryRowContext(ctx, `SELECT price_micros FROM als_distributor_package_assignments WHERE distributor_user_id = ? AND target_user_id = ?;`, distributorID, targetUserID).Scan(&assignmentPriceMicros); err != nil {
		t.Fatalf("query distributor assignment price: %v", err)
	}
	if assignmentPriceMicros != 1000000 {
		t.Fatalf("expected distributor assignment price snapshot 1000000, got %d", assignmentPriceMicros)
	}

	statsReq := httptest.NewRequest(http.MethodGet, "/distributor/stats", nil)
	setBearerAuth(statsReq, distributorSessionToken)
	statsRec := httptest.NewRecorder()
	mux.ServeHTTP(statsRec, statsReq)
	if statsRec.Code != http.StatusOK {
		t.Fatalf("expected distributor stats status %d, got %d body=%s", http.StatusOK, statsRec.Code, statsRec.Body.String())
	}
	var statsPayload distributorAssignmentStatsResponse
	if err := json.NewDecoder(statsRec.Body).Decode(&statsPayload); err != nil {
		t.Fatalf("decode distributor stats: %v", err)
	}
	if statsPayload.Totals.AssignmentCount != 1 || statsPayload.Totals.UniqueUserCount != 1 || statsPayload.Totals.TotalPriceMicros != 1000000 {
		t.Fatalf("unexpected distributor stats totals: %+v", statsPayload.Totals)
	}
	if len(statsPayload.Daily) != 1 || statsPayload.Daily[0].AssignmentCount != 1 || statsPayload.Daily[0].TotalPriceMicros != 1000000 {
		t.Fatalf("unexpected distributor daily stats: %+v", statsPayload.Daily)
	}
	if len(statsPayload.Packages) != 1 || statsPayload.Packages[0].TierCode != "distributor-package" || statsPayload.Packages[0].TotalPriceMicros != 1000000 {
		t.Fatalf("unexpected distributor package stats: %+v", statsPayload.Packages)
	}
	if len(statsPayload.Users) != 1 || statsPayload.Users[0].UserID != targetUserID || statsPayload.Users[0].TotalPriceMicros != 1000000 {
		t.Fatalf("unexpected distributor user stats: %+v", statsPayload.Users)
	}

	adminStatsReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/distributor/stats?distributor_user_id=%d", distributorID), nil)
	setBearerAuth(adminStatsReq, adminSessionToken)
	adminStatsRec := httptest.NewRecorder()
	mux.ServeHTTP(adminStatsRec, adminStatsReq)
	if adminStatsRec.Code != http.StatusOK {
		t.Fatalf("expected admin distributor stats status %d, got %d body=%s", http.StatusOK, adminStatsRec.Code, adminStatsRec.Body.String())
	}
	var adminStatsPayload distributorAssignmentStatsResponse
	if err := json.NewDecoder(adminStatsRec.Body).Decode(&adminStatsPayload); err != nil {
		t.Fatalf("decode admin distributor stats: %v", err)
	}
	if adminStatsPayload.Totals.AssignmentCount != 1 || adminStatsPayload.Totals.DistributorCount != 1 || adminStatsPayload.Totals.TotalPriceMicros != 1000000 {
		t.Fatalf("unexpected admin distributor stats totals: %+v", adminStatsPayload.Totals)
	}
	if len(adminStatsPayload.Distributors) != 1 || adminStatsPayload.Distributors[0].DistributorUserID != distributorID || adminStatsPayload.Distributors[0].TotalPriceMicros != 1000000 {
		t.Fatalf("unexpected admin distributor breakdown: %+v", adminStatsPayload.Distributors)
	}

	forbiddenReq := httptest.NewRequest(http.MethodPost, "/distributor/assign-package", bytes.NewReader([]byte(`{"email":"other-distributor-user@example.com","tier_code":"distributor-package"}`)))
	forbiddenReq.Header.Set("Content-Type", "application/json")
	setBearerAuth(forbiddenReq, distributorSessionToken)
	forbiddenRec := httptest.NewRecorder()
	mux.ServeHTTP(forbiddenRec, forbiddenReq)
	if forbiddenRec.Code != http.StatusForbidden {
		t.Fatalf("expected unbound assign status %d, got %d body=%s", http.StatusForbidden, forbiddenRec.Code, forbiddenRec.Body.String())
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
		"is_visible":    true,
		"is_published":  true,
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
		Code        string   `json:"code"`
		Name        string   `json:"name"`
		Level       string   `json:"level"`
		GroupIDs    []int64  `json:"group_ids"`
		PriceMicros int64    `json:"price_micros"`
		ValueType   string   `json:"value_type"`
		ValueAmount int64    `json:"value_amount"`
		Description string   `json:"description"`
		Features    []string `json:"features"`
		IsEnabled   bool     `json:"is_enabled"`
		IsVisible   bool     `json:"is_visible"`
		IsPublished bool     `json:"is_published"`
	}
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Code != "pro-monthly" {
		t.Fatalf("expected code pro-monthly, got %s", created.Code)
	}
	if created.Level != "admin" {
		t.Fatalf("expected default level admin, got %s", created.Level)
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
	if !created.IsVisible {
		t.Fatal("expected is_visible true")
	}
	if !created.IsPublished {
		t.Fatal("expected is_published true")
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
			IsVisible   bool     `json:"is_visible"`
			IsPublished bool     `json:"is_published"`
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
		"is_visible":    false,
		"is_published":  false,
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
		Level       string   `json:"level"`
		PriceMicros int64    `json:"price_micros"`
		ValueAmount int64    `json:"value_amount"`
		IsEnabled   bool     `json:"is_enabled"`
		IsVisible   bool     `json:"is_visible"`
		IsPublished bool     `json:"is_published"`
		Features    []string `json:"features"`
	}
	if err := json.NewDecoder(updateRec.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.PriceMicros != 19900000 {
		t.Fatalf("expected updated price_micros 19900000, got %d", updated.PriceMicros)
	}
	if updated.Level != "admin" {
		t.Fatalf("expected updated level to remain admin, got %s", updated.Level)
	}
	if updated.ValueAmount != 90 {
		t.Fatalf("expected updated value_amount 90, got %d", updated.ValueAmount)
	}
	if updated.IsEnabled {
		t.Fatal("expected is_enabled false after update")
	}
	if updated.IsVisible {
		t.Fatal("expected is_visible false after update")
	}
	if updated.IsPublished {
		t.Fatal("expected is_published false after update")
	}
	if len(updated.Features) != 2 {
		t.Fatalf("expected 2 features after update, got %d", len(updated.Features))
	}
}

func TestDistributorPackageAccessOnlyIncludesDistributorLevel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})
	_, distributorSessionToken := createUserViaAPI(t, mux, "distributor-packages@example.com", "Package Distributor", "distributor", "test-admin-secret")

	if _, err := database.ExecContext(ctx, `
		INSERT INTO als_tiers(code, name, level, price_micros, value_type, value_amount, description, features_json, is_enabled, is_visible, is_published)
		VALUES
			('admin-only', 'Admin Only', 'admin', 1000000, 'days', 30, '', '[]', 1, 1, 1),
			('distributor-only', 'Distributor Only', 'distributor', 1000000, 'days', 30, '', '[]', 1, 1, 1),
			('disabled-distributor', 'Disabled Distributor', 'distributor', 1000000, 'days', 30, '', '[]', 0, 1, 1),
			('unpublished-admin', 'Unpublished Admin', 'admin', 1000000, 'days', 30, '', '[]', 1, 1, 0);
	`); err != nil {
		t.Fatalf("seed packages: %v", err)
	}

	adminListReq := httptest.NewRequest(http.MethodGet, "/admin/packages", nil)
	setBearerAuth(adminListReq, distributorSessionToken)
	adminListRec := httptest.NewRecorder()
	mux.ServeHTTP(adminListRec, adminListReq)
	if adminListRec.Code != http.StatusForbidden {
		t.Fatalf("expected admin package list status %d, got %d: %s", http.StatusForbidden, adminListRec.Code, adminListRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/distributor/packages", nil)
	setBearerAuth(listReq, distributorSessionToken)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected distributor package list status %d, got %d: %s", http.StatusOK, listRec.Code, listRec.Body.String())
	}

	var listPayload struct {
		Packages []struct {
			Code  string `json:"code"`
			Level string `json:"level"`
		} `json:"packages"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode distributor package list: %v", err)
	}
	seen := map[string]string{}
	for _, pkg := range listPayload.Packages {
		seen[pkg.Code] = pkg.Level
	}
	if len(seen) != 1 {
		t.Fatalf("expected distributor to see 1 assignable package, got %+v", listPayload.Packages)
	}
	if _, ok := seen["admin-only"]; ok {
		t.Fatalf("expected distributor admin package to be hidden, got %+v", listPayload.Packages)
	}
	if seen["distributor-only"] != "distributor" {
		t.Fatalf("expected distributor to see distributor package, got %+v", listPayload.Packages)
	}
	if _, ok := seen["disabled-distributor"]; ok {
		t.Fatalf("expected disabled distributor package to be hidden, got %+v", listPayload.Packages)
	}
	if _, ok := seen["unpublished-admin"]; ok {
		t.Fatalf("expected unpublished admin package to be hidden, got %+v", listPayload.Packages)
	}

	adminDetailReq := httptest.NewRequest(http.MethodGet, "/admin/packages/admin-only", nil)
	setBearerAuth(adminDetailReq, distributorSessionToken)
	adminDetailRec := httptest.NewRecorder()
	mux.ServeHTTP(adminDetailRec, adminDetailReq)
	if adminDetailRec.Code != http.StatusForbidden {
		t.Fatalf("expected admin package detail status %d, got %d: %s", http.StatusForbidden, adminDetailRec.Code, adminDetailRec.Body.String())
	}

}

func TestAdminUpdateUserRoleControlsLocalRole(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})
	adminID, adminSessionToken := createUserViaAPI(t, mux, "role-admin@example.com", "Role Admin", "admin", "test-admin-secret")
	_, distributorSessionToken := createUserViaAPI(t, mux, "role-distributor@example.com", "Role Distributor", "distributor", "test-admin-secret")
	targetID := createUser(t, ctx, database, "role-target@example.com", "Role Target", "user")

	body := []byte(`{"email":"role-target@example.com","role":"distributor"}`)
	req := httptest.NewRequest(http.MethodPut, "/admin/users/role", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setBearerAuth(req, adminSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected role update status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var updatedRole string
	if err := database.QueryRowContext(ctx, `SELECT role FROM als_users WHERE id = ?;`, targetID).Scan(&updatedRole); err != nil {
		t.Fatalf("query target role: %v", err)
	}
	if updatedRole != "distributor" {
		t.Fatalf("expected target role distributor, got %s", updatedRole)
	}

	distributorReq := httptest.NewRequest(http.MethodPut, "/admin/users/role", bytes.NewReader([]byte(`{"email":"role-target@example.com","role":"user"}`)))
	distributorReq.Header.Set("Content-Type", "application/json")
	setBearerAuth(distributorReq, distributorSessionToken)
	distributorRec := httptest.NewRecorder()
	mux.ServeHTTP(distributorRec, distributorReq)
	if distributorRec.Code != http.StatusForbidden {
		t.Fatalf("expected distributor role update status %d, got %d: %s", http.StatusForbidden, distributorRec.Code, distributorRec.Body.String())
	}

	lastAdminReq := httptest.NewRequest(http.MethodPut, "/admin/users/role", bytes.NewReader([]byte(fmt.Sprintf(`{"user_id":%d,"role":"user"}`, adminID))))
	lastAdminReq.Header.Set("Content-Type", "application/json")
	setBearerAuth(lastAdminReq, adminSessionToken)
	lastAdminRec := httptest.NewRecorder()
	mux.ServeHTTP(lastAdminRec, lastAdminReq)
	if lastAdminRec.Code != http.StatusConflict {
		t.Fatalf("expected last admin guard status %d, got %d: %s", http.StatusConflict, lastAdminRec.Code, lastAdminRec.Body.String())
	}
}

func TestAdminUpdateUserRoleImportsUpstreamUserOverlay(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)

	upstreamHits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		upstreamHits++
		if req.Method != http.MethodGet || req.URL.Path != "/api/v1/admin/users/901" {
			t.Fatalf("unexpected upstream request: %s %s", req.Method, req.URL.Path)
		}
		if got := req.Header.Get("x-api-key"); got != "admin-key-123" {
			t.Fatalf("expected admin api key, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"id":901,"email":"upstream-shadow@example.com","name":"Upstream Shadow"}}`))
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClientWithOptions(upstream.URL, &http.Client{}, proxy.ClientOptions{AdminAPIKey: "admin-key-123"})
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{
		AdminBootstrapSecret: "test-admin-secret",
		ProxyClient:          proxyClient,
	})
	_, adminSessionToken := createUserViaAPI(t, mux, "role-import-admin@example.com", "Role Import Admin", "admin", "test-admin-secret")

	req := httptest.NewRequest(http.MethodPut, "/admin/users/role", bytes.NewReader([]byte(`{"user_id":901,"role":"distributor"}`)))
	req.Header.Set("Content-Type", "application/json")
	setBearerAuth(req, adminSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected role import status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if upstreamHits != 1 {
		t.Fatalf("expected one upstream user lookup, got %d", upstreamHits)
	}

	var payload struct {
		ID            int64  `json:"id"`
		Sub2APIUserID int64  `json:"sub2api_user_id"`
		Role          string `json:"role"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode role import response: %v", err)
	}
	if payload.ID != 901 || payload.Sub2APIUserID != 901 || payload.Role != "distributor" {
		t.Fatalf("unexpected role import response: %+v", payload)
	}

	var (
		localRole      string
		upstreamUserID int64
	)
	if err := database.QueryRowContext(ctx, `
		SELECT u.role, tok.upstream_user_id
		FROM als_users u
		JOIN als_sub2api_auth_tokens tok ON tok.user_id = u.id
		WHERE u.email = 'upstream-shadow@example.com';
	`).Scan(&localRole, &upstreamUserID); err != nil {
		t.Fatalf("query imported role overlay: %v", err)
	}
	if localRole != "distributor" || upstreamUserID != 901 {
		t.Fatalf("unexpected imported role overlay role=%s upstream=%d", localRole, upstreamUserID)
	}
}

func TestPublicPackagesReturnsOnlyVisible(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)

	// Insert als_tiers directly — one visible, one hidden.
	_, err := database.ExecContext(ctx, `INSERT INTO als_tiers(code, name, price_micros, value_type, value_amount, description, features_json, is_enabled, is_visible, is_published) VALUES ('free', 'Free', 0, 'days', 30, 'Perfect for exploring.', '["2 Global Nodes"]', 1, 1, 1);`)
	if err != nil {
		t.Fatalf("insert free tier: %v", err)
	}
	_, err = database.ExecContext(ctx, `INSERT INTO als_tiers(code, name, price_micros, value_type, value_amount, description, features_json, is_enabled, is_visible, is_published) VALUES ('hidden', 'Hidden', 99000000, 'days', 365, 'Hidden tier.', '["Everything"]', 1, 0, 1);`)
	if err != nil {
		t.Fatalf("insert hidden tier: %v", err)
	}
	_, err = database.ExecContext(ctx, `INSERT INTO als_tiers(code, name, level, price_micros, value_type, value_amount, description, features_json, is_enabled, is_visible, is_published) VALUES ('distributor-visible', 'Distributor Visible', 'distributor', 99000000, 'days', 365, 'Distributor tier.', '["Everything"]', 1, 1, 1);`)
	if err != nil {
		t.Fatalf("insert distributor tier: %v", err)
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
