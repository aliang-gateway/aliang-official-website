package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSubscriptionWithTierCode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "starter", "Starter")
	itemID := insertServiceItem(t, ctx, database, "tokens_in", "Input Tokens", "token")
	insertTierDefaultItem(t, ctx, database, tierID, itemID, 1200)

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)
	userID, sessionToken := createUserViaAPI(t, mux, "user1@example.com", "User One", "user", "")

	req := httptest.NewRequest(http.MethodPost, "/subscription", bytes.NewReader([]byte(`{"tier_code":"starter"}`)))
	setBearerAuth(req, sessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var payload struct {
		Subscription struct {
			TierCode string `json:"tier_code"`
			TierName string `json:"tier_name"`
			Quotas   []struct {
				ServiceItemCode string `json:"service_item_code"`
				IncludedUnits   int64  `json:"included_units"`
			} `json:"quotas"`
		} `json:"subscription"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Subscription.TierCode != "starter" || payload.Subscription.TierName != "Starter" {
		t.Fatalf("unexpected subscription identity: %+v", payload.Subscription)
	}
	if len(payload.Subscription.Quotas) != 1 {
		t.Fatalf("expected one quota, got %d", len(payload.Subscription.Quotas))
	}
	if payload.Subscription.Quotas[0].ServiceItemCode != "tokens_in" || payload.Subscription.Quotas[0].IncludedUnits != 1200 {
		t.Fatalf("unexpected quota payload: %+v", payload.Subscription.Quotas[0])
	}

	var activeCount int64
	err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM als_subscriptions
		WHERE user_id = ? AND status = 'active' AND ended_at IS NULL;
	`, userID).Scan(&activeCount)
	if err != nil {
		t.Fatalf("query active als_subscriptions: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active subscription, got %d", activeCount)
	}
}

func TestSubscriptionOverrideReflectedInGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "pro", "Pro")
	itemID := insertServiceItem(t, ctx, database, "tokens_out", "Output Tokens", "token")
	insertTierDefaultItem(t, ctx, database, tierID, itemID, 500)

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)
	_, sessionToken := createUserViaAPI(t, mux, "user2@example.com", "User Two", "user", "")

	createBody := []byte(`{"tier_code":"pro","overrides":[{"service_item_code":"tokens_out","included_units":750}]}`)
	createReq := httptest.NewRequest(http.MethodPost, "/subscription", bytes.NewReader(createBody))
	setBearerAuth(createReq, sessionToken)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d", http.StatusCreated, createRec.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/subscription", nil)
	setBearerAuth(getReq, sessionToken)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d", http.StatusOK, getRec.Code)
	}

	var payload struct {
		Subscription struct {
			TierCode string `json:"tier_code"`
			TierName string `json:"tier_name"`
			Quotas   []struct {
				ServiceItemCode string `json:"service_item_code"`
				IncludedUnits   int64  `json:"included_units"`
			} `json:"quotas"`
		} `json:"subscription"`
	}
	if err := json.NewDecoder(getRec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode get response: %v", err)
	}

	if payload.Subscription.TierCode != "pro" || payload.Subscription.TierName != "Pro" {
		t.Fatalf("unexpected subscription in get response: %+v", payload.Subscription)
	}
	if len(payload.Subscription.Quotas) != 1 {
		t.Fatalf("expected one quota, got %d", len(payload.Subscription.Quotas))
	}
	if payload.Subscription.Quotas[0].ServiceItemCode != "tokens_out" || payload.Subscription.Quotas[0].IncludedUnits != 750 {
		t.Fatalf("expected override included_units=750, got %+v", payload.Subscription.Quotas[0])
	}
}

func TestReplacingActiveSubscriptionEndsPrevious(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierBasicID := insertTier(t, ctx, database, "basic", "Basic")
	tierProID := insertTier(t, ctx, database, "pro", "Pro")
	itemID := insertServiceItem(t, ctx, database, "requests", "Requests", "request")
	insertTierDefaultItem(t, ctx, database, tierBasicID, itemID, 100)
	insertTierDefaultItem(t, ctx, database, tierProID, itemID, 1000)

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)
	userID, sessionToken := createUserViaAPI(t, mux, "user3@example.com", "User Three", "user", "")

	firstReq := httptest.NewRequest(http.MethodPost, "/subscription", bytes.NewReader([]byte(`{"tier_code":"basic"}`)))
	setBearerAuth(firstReq, sessionToken)
	firstRec := httptest.NewRecorder()
	mux.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected first create status %d, got %d", http.StatusCreated, firstRec.Code)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/subscription", bytes.NewReader([]byte(`{"tier_code":"pro"}`)))
	setBearerAuth(secondReq, sessionToken)
	secondRec := httptest.NewRecorder()
	mux.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusCreated {
		t.Fatalf("expected second create status %d, got %d", http.StatusCreated, secondRec.Code)
	}

	var (
		status string
		ended  sql.NullString
	)
	err := database.QueryRowContext(ctx, `
		SELECT status, ended_at
		FROM als_subscriptions
		WHERE user_id = ?
		ORDER BY started_at ASC, id ASC
		LIMIT 1;
	`, userID).Scan(&status, &ended)
	if err != nil {
		t.Fatalf("query oldest subscription: %v", err)
	}
	if status != "ended" || !ended.Valid {
		t.Fatalf("expected first subscription to be ended with ended_at, got status=%q ended_at_valid=%v", status, ended.Valid)
	}

	var activeCount int64
	err = database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM als_subscriptions
		WHERE user_id = ? AND status = 'active' AND ended_at IS NULL;
	`, userID).Scan(&activeCount)
	if err != nil {
		t.Fatalf("count active als_subscriptions: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected one active subscription after replacement, got %d", activeCount)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/subscription", nil)
	setBearerAuth(getReq, sessionToken)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d", http.StatusOK, getRec.Code)
	}

	var payload struct {
		Subscription struct {
			TierCode string `json:"tier_code"`
			TierName string `json:"tier_name"`
		} `json:"subscription"`
	}
	if err := json.NewDecoder(getRec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if payload.Subscription.TierCode != "pro" || payload.Subscription.TierName != "Pro" {
		t.Fatalf("expected active pro subscription, got %+v", payload.Subscription)
	}
}

func insertUser(t *testing.T, ctx context.Context, database *sql.DB, email, name, role string) int64 {
	t.Helper()

	result, err := database.ExecContext(ctx, `INSERT INTO als_users(email, name, role) VALUES (?, ?, ?);`, email, name, role)
	if err != nil {
		t.Fatalf("insert user error = %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("insert user LastInsertId error = %v", err)
	}

	return id
}
