package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ai-api-portal/backend/internal/apikey"
)

func TestAIRequestRequiresAPIKeyHeader(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	userID := insertUser(t, ctx, database, "protected1@example.com", "Protected One", "user")
	tierID := insertTier(t, ctx, database, "starter", "Starter")
	itemID := insertServiceItem(t, ctx, database, "ai_requests", "AI Requests", "request")
	insertTierDefaultItem(t, ctx, database, tierID, itemID, 10)
	insertActiveSubscription(t, ctx, database, userID, tierID, "2026-01-01 00:00:00")

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/request", bytes.NewReader([]byte(`{"service_item_code":"ai_requests","quantity":1}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestAIRequestRejectsInvalidAndRevokedAPIKeys(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	userID := insertUser(t, ctx, database, "protected2@example.com", "Protected Two", "user")
	tierID := insertTier(t, ctx, database, "starter", "Starter")
	itemID := insertServiceItem(t, ctx, database, "ai_requests", "AI Requests", "request")
	insertTierDefaultItem(t, ctx, database, tierID, itemID, 10)
	insertActiveSubscription(t, ctx, database, userID, tierID, "2026-01-01 00:00:00")

	apiSvc := apikey.NewService(database)
	created, err := apiSvc.CreateKey(ctx, userID, "for-protected")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	if _, err := database.ExecContext(ctx, `UPDATE als_api_keys SET revoked_at = CURRENT_TIMESTAMP WHERE id = ?;`, created.ID); err != nil {
		t.Fatalf("revoke api key directly: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	invalidReq := httptest.NewRequest(http.MethodPost, "/api/ai/request", bytes.NewReader([]byte(`{"service_item_code":"ai_requests","quantity":1}`)))
	invalidReq.Header.Set("X-API-Key", "ak_invalid")
	invalidRec := httptest.NewRecorder()
	mux.ServeHTTP(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected invalid key status %d, got %d", http.StatusUnauthorized, invalidRec.Code)
	}

	revokedReq := httptest.NewRequest(http.MethodPost, "/api/ai/request", bytes.NewReader([]byte(`{"service_item_code":"ai_requests","quantity":1}`)))
	revokedReq.Header.Set("X-API-Key", created.APIKey)
	revokedRec := httptest.NewRecorder()
	mux.ServeHTTP(revokedRec, revokedReq)
	if revokedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked key status %d, got %d", http.StatusUnauthorized, revokedRec.Code)
	}
}

func TestAIRequestQuotaExceeded(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	userID := insertUser(t, ctx, database, "protected3@example.com", "Protected Three", "user")
	tierID := insertTier(t, ctx, database, "starter", "Starter")
	itemID := insertServiceItem(t, ctx, database, "ai_requests", "AI Requests", "request")
	insertTierDefaultItem(t, ctx, database, tierID, itemID, 2)
	subscriptionID := insertActiveSubscription(t, ctx, database, userID, tierID, "2026-01-01 00:00:00")

	apiSvc := apikey.NewService(database)
	created, err := apiSvc.CreateKey(ctx, userID, "quota-check")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	insertUsageRecord(t, ctx, database, userID, created.ID, itemID, 2, "2026-01-02 00:00:00")
	insertUsageRecord(t, ctx, database, userID, created.ID, itemID, 100, "2025-12-31 23:59:59")

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/request", bytes.NewReader([]byte(`{"service_item_code":"ai_requests","quantity":1}`)))
	req.Header.Set("X-API-Key", created.APIKey)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}

	var payload struct {
		Allowed        bool  `json:"allowed"`
		IncludedUnits  int64 `json:"included_units"`
		UsedUnits      int64 `json:"used_units"`
		RemainingUnits int64 `json:"remaining_units"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode quota exceeded payload: %v", err)
	}
	if payload.Allowed {
		t.Fatalf("expected allowed=false, got true")
	}
	if payload.IncludedUnits != 2 || payload.UsedUnits != 2 || payload.RemainingUnits != 0 {
		t.Fatalf("unexpected quota payload: %+v", payload)
	}

	var usageCount int64
	err = database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM als_usage_records
		WHERE user_id = ? AND service_item_id = ? AND usage_timestamp >= (SELECT started_at FROM als_subscriptions WHERE id = ?);
	`, userID, itemID, subscriptionID).Scan(&usageCount)
	if err != nil {
		t.Fatalf("count usage after quota deny: %v", err)
	}
	if usageCount != 1 {
		t.Fatalf("expected no additional usage records in active window, got count=%d", usageCount)
	}
}

func TestAIRequestSuccessRecordsUsage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	userID := insertUser(t, ctx, database, "protected4@example.com", "Protected Four", "user")
	tierID := insertTier(t, ctx, database, "pro", "Pro")
	itemID := insertServiceItem(t, ctx, database, "ai_requests", "AI Requests", "request")
	insertTierDefaultItem(t, ctx, database, tierID, itemID, 5)
	subscriptionID := insertActiveSubscription(t, ctx, database, userID, tierID, "2026-01-01 00:00:00")

	apiSvc := apikey.NewService(database)
	created, err := apiSvc.CreateKey(ctx, userID, "allow-check")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	insertUsageRecord(t, ctx, database, userID, created.ID, itemID, 2, "2026-01-03 00:00:00")

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/request", bytes.NewReader([]byte(`{"service_item_code":"ai_requests","quantity":2}`)))
	req.Header.Set("X-API-Key", created.APIKey)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload struct {
		Allowed        bool  `json:"allowed"`
		IncludedUnits  int64 `json:"included_units"`
		UsedUnits      int64 `json:"used_units"`
		RemainingUnits int64 `json:"remaining_units"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode success payload: %v", err)
	}
	if !payload.Allowed {
		t.Fatalf("expected allowed=true, got false")
	}
	if payload.IncludedUnits != 5 || payload.UsedUnits != 4 || payload.RemainingUnits != 1 {
		t.Fatalf("unexpected success payload: %+v", payload)
	}

	var (
		totalUsed int64
		lastQty   int64
		lastKeyID sql.NullInt64
	)
	err = database.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(quantity), 0)
		FROM als_usage_records
		WHERE user_id = ? AND service_item_id = ? AND usage_timestamp >= (SELECT started_at FROM als_subscriptions WHERE id = ?);
	`, userID, itemID, subscriptionID).Scan(&totalUsed)
	if err != nil {
		t.Fatalf("sum usage in active window: %v", err)
	}
	if totalUsed != 4 {
		t.Fatalf("expected total used 4 after success, got %d", totalUsed)
	}

	err = database.QueryRowContext(ctx, `
		SELECT quantity, api_key_id
		FROM als_usage_records
		WHERE user_id = ? AND service_item_id = ?
		ORDER BY id DESC
		LIMIT 1;
	`, userID, itemID).Scan(&lastQty, &lastKeyID)
	if err != nil {
		t.Fatalf("read latest usage record: %v", err)
	}
	if lastQty != 2 {
		t.Fatalf("expected latest usage quantity 2, got %d", lastQty)
	}
	if !lastKeyID.Valid || lastKeyID.Int64 != created.ID {
		t.Fatalf("expected latest usage api_key_id=%d, got valid=%v id=%d", created.ID, lastKeyID.Valid, lastKeyID.Int64)
	}
}

func insertActiveSubscription(t *testing.T, ctx context.Context, database *sql.DB, userID, tierID int64, startedAt string) int64 {
	t.Helper()

	result, err := database.ExecContext(ctx, `
		INSERT INTO als_subscriptions(user_id, tier_id, status, started_at)
		VALUES (?, ?, 'active', ?);
	`, userID, tierID, startedAt)
	if err != nil {
		t.Fatalf("insert active subscription error = %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("insert active subscription LastInsertId error = %v", err)
	}

	return id
}

func insertUsageRecord(t *testing.T, ctx context.Context, database *sql.DB, userID, apiKeyID, serviceItemID, quantity int64, usageTimestamp string) {
	t.Helper()

	_, err := database.ExecContext(ctx, `
		INSERT INTO als_usage_records(user_id, api_key_id, service_item_id, quantity, usage_timestamp)
		VALUES (?, ?, ?, ?, ?);
	`, userID, apiKeyID, serviceItemID, quantity, usageTimestamp)
	if err != nil {
		t.Fatalf("insert usage record error = %v", err)
	}
}
