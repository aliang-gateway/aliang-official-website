package fulfillment

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/proxy"
)

func TestCreateOrLoadJobByIdempotency_ReplayReturnsExistingJobWithoutDuplicates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewServiceWithDialect(database, testDialect())

	userID := createUser(t, ctx, database, "fulfillment-replay@example.com", "Replay User", "user")
	subscriptionID := createSubscription(t, ctx, database, userID, "starter", "Starter")

	input := &CreateJobInput{
		UserID:         &userID,
		SubscriptionID: &subscriptionID,
		EventType:      "payment_succeeded",
		PayloadJSON:    `{"source":"stripe","invoice_id":"in_1"}`,
		IdempotencyKey: "idem-1",
	}

	first, err := service.CreateOrLoadJobByIdempotency(ctx, input)
	if err != nil {
		t.Fatalf("CreateOrLoadJobByIdempotency() first error = %v", err)
	}
	if first.ID <= 0 {
		t.Fatalf("expected created job to have positive ID")
	}
	if first.Status != StatusPaidUnfulfilled {
		t.Fatalf("expected initial status %q, got %q", StatusPaidUnfulfilled, first.Status)
	}

	second, err := service.CreateOrLoadJobByIdempotency(ctx, input)
	if err != nil {
		t.Fatalf("CreateOrLoadJobByIdempotency() second error = %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected replay to return same job id, got first=%d second=%d", first.ID, second.ID)
	}

	assertCount(t, ctx, database, `SELECT COUNT(*) FROM als_fulfillment_jobs WHERE idempotency_key = ?;`, "idem-1", 1)
	assertCount(t, ctx, database, `SELECT COUNT(*) FROM als_fulfillment_events WHERE fulfillment_job_id = ?;`, first.ID, 1)

	events, err := service.ListEventsByJobID(ctx, first.ID)
	if err != nil {
		t.Fatalf("ListEventsByJobID() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != EventTypeJobCreated {
		t.Fatalf("expected first event type %q, got %q", EventTypeJobCreated, events[0].EventType)
	}
}

func TestCreateOrLoadJobByIdempotency_ConflictingReuseFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewServiceWithDialect(database, testDialect())

	userID := createUser(t, ctx, database, "fulfillment-conflict@example.com", "Conflict User", "user")
	subscriptionID := createSubscription(t, ctx, database, userID, "pro", "Pro")

	firstInput := &CreateJobInput{
		UserID:         &userID,
		SubscriptionID: &subscriptionID,
		EventType:      "payment_succeeded",
		PayloadJSON:    `{"source":"stripe","invoice_id":"in_1"}`,
		IdempotencyKey: "idem-conflict",
	}
	if _, err := service.CreateOrLoadJobByIdempotency(ctx, firstInput); err != nil {
		t.Fatalf("initial CreateOrLoadJobByIdempotency() error = %v", err)
	}

	conflicting := &CreateJobInput{
		UserID:         &userID,
		SubscriptionID: &subscriptionID,
		EventType:      "payment_succeeded",
		PayloadJSON:    `{"source":"stripe","invoice_id":"in_2"}`,
		IdempotencyKey: "idem-conflict",
	}
	_, err := service.CreateOrLoadJobByIdempotency(ctx, conflicting)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected ErrIdempotencyConflict, got %v", err)
	}

	assertCount(t, ctx, database, `SELECT COUNT(*) FROM als_fulfillment_jobs WHERE idempotency_key = ?;`, "idem-conflict", 1)
	assertCount(t, ctx, database, `SELECT COUNT(*) FROM als_fulfillment_events;`, nil, 1)
}

func TestTransitionJob_LegalTransitionsAppendEvents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewServiceWithDialect(database, testDialect())

	userID := createUser(t, ctx, database, "fulfillment-transitions@example.com", "Transition User", "user")
	subscriptionID := createSubscription(t, ctx, database, userID, "growth", "Growth")

	job, err := service.CreateOrLoadJobByIdempotency(ctx, &CreateJobInput{
		UserID:         &userID,
		SubscriptionID: &subscriptionID,
		EventType:      "payment_succeeded",
		PayloadJSON:    `{"source":"stripe","invoice_id":"in_3"}`,
		IdempotencyKey: "idem-transition",
	})
	if err != nil {
		t.Fatalf("CreateOrLoadJobByIdempotency() error = %v", err)
	}

	availableAt := time.Now().UTC().Add(15 * time.Minute)
	retryCount := 1
	errorMessage := "temporary provider outage"
	failed, err := service.TransitionJob(ctx, job.ID, &TransitionInput{
		Status:       StatusFailedRetryable,
		ErrorMessage: &errorMessage,
		EventType:    "transition_failed_retryable",
		EventPayload: ptr(`{"reason":"provider_outage"}`),
		AvailableAt:  &availableAt,
		RetryCount:   &retryCount,
	})
	if err != nil {
		t.Fatalf("TransitionJob() to failed_retryable error = %v", err)
	}
	if failed.Status != StatusFailedRetryable {
		t.Fatalf("expected status %q, got %q", StatusFailedRetryable, failed.Status)
	}
	if failed.ErrorMessage == nil || *failed.ErrorMessage != errorMessage {
		t.Fatalf("expected error_message %q, got %+v", errorMessage, failed.ErrorMessage)
	}
	if failed.RetryCount != retryCount {
		t.Fatalf("expected retry_count %d, got %d", retryCount, failed.RetryCount)
	}

	startedAt := time.Now().UTC().Add(20 * time.Minute)
	finishedAt := startedAt.Add(2 * time.Minute)
	fulfilled, err := service.TransitionJob(ctx, job.ID, &TransitionInput{
		Status:       StatusFulfilled,
		EventType:    "transition_fulfilled",
		EventPayload: ptr(`{"delivery":"ok"}`),
		StartedAt:    &startedAt,
		FinishedAt:   &finishedAt,
	})
	if err != nil {
		t.Fatalf("TransitionJob() to fulfilled error = %v", err)
	}
	if fulfilled.Status != StatusFulfilled {
		t.Fatalf("expected status %q, got %q", StatusFulfilled, fulfilled.Status)
	}
	if fulfilled.StartedAt == nil || fulfilled.FinishedAt == nil {
		t.Fatalf("expected started_at and finished_at to be populated")
	}

	events, err := service.ListEventsByJobID(ctx, job.ID)
	if err != nil {
		t.Fatalf("ListEventsByJobID() error = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events (create + two transitions), got %d", len(events))
	}
	if events[0].EventType != EventTypeJobCreated || events[1].EventType != "transition_failed_retryable" || events[2].EventType != "transition_fulfilled" {
		t.Fatalf("unexpected event sequence: %+v", events)
	}
}

func TestTransitionJob_IllegalTransitionRejected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewServiceWithDialect(database, testDialect())

	userID := createUser(t, ctx, database, "fulfillment-illegal@example.com", "Illegal User", "user")
	subscriptionID := createSubscription(t, ctx, database, userID, "team", "Team")

	job, err := service.CreateOrLoadJobByIdempotency(ctx, &CreateJobInput{
		UserID:         &userID,
		SubscriptionID: &subscriptionID,
		EventType:      "payment_succeeded",
		PayloadJSON:    `{"source":"stripe","invoice_id":"in_4"}`,
		IdempotencyKey: "idem-illegal",
	})
	if err != nil {
		t.Fatalf("CreateOrLoadJobByIdempotency() error = %v", err)
	}

	if _, err := service.TransitionJob(ctx, job.ID, &TransitionInput{Status: StatusFulfilled, EventType: "transition_fulfilled"}); err != nil {
		t.Fatalf("TransitionJob() to fulfilled should succeed, got %v", err)
	}

	_, err = service.TransitionJob(ctx, job.ID, &TransitionInput{Status: StatusPaidUnfulfilled, EventType: "transition_back"})
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}

	assertCount(t, ctx, database, `SELECT COUNT(*) FROM als_fulfillment_events WHERE fulfillment_job_id = ?;`, job.ID, 2)
	stored, err := service.GetJobByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJobByID() error = %v", err)
	}
	if stored.Status != StatusFulfilled {
		t.Fatalf("expected stored status to remain %q, got %q", StatusFulfilled, stored.Status)
	}
}

func TestValidationPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewServiceWithDialect(database, testDialect())

	if _, err := service.CreateOrLoadJobByIdempotency(ctx, nil); err == nil {
		t.Fatalf("expected error for nil create input")
	}

	if _, err := service.CreateOrLoadJobByIdempotency(ctx, &CreateJobInput{EventType: "payment_succeeded"}); err == nil {
		t.Fatalf("expected error for missing idempotency key")
	}

	if _, err := service.TransitionJob(ctx, 1, nil); err == nil {
		t.Fatalf("expected error for nil transition input")
	}

	if _, err := service.TransitionJob(ctx, 1, &TransitionInput{Status: "", EventType: "event"}); err == nil {
		t.Fatalf("expected error for missing transition status")
	}

	if _, err := service.ListEventsByJobID(ctx, 0); err == nil {
		t.Fatalf("expected error for invalid job id in ListEventsByJobID")
	}

	if _, err := service.GetJobByID(ctx, 0); err == nil {
		t.Fatalf("expected error for invalid job id in GetJobByID")
	}
}

func TestApplyCreateAndRedeemResult_MapsSuccessAndIdempotencyErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewServiceWithDialect(database, testDialect())

	userID := createUser(t, ctx, database, "fulfillment-apply@example.com", "Apply User", "user")
	subscriptionID := createSubscription(t, ctx, database, userID, "apply", "Apply")

	newJob := func(key string) *Job {
		t.Helper()
		job, err := service.CreateOrLoadJobByIdempotency(ctx, &CreateJobInput{
			UserID:         &userID,
			SubscriptionID: &subscriptionID,
			EventType:      "payment_succeeded",
			PayloadJSON:    `{"source":"portal"}`,
			IdempotencyKey: key,
		})
		if err != nil {
			t.Fatalf("CreateOrLoadJobByIdempotency() error = %v", err)
		}
		return job
	}

	successJob := newJob("apply-success")
	fulfilled, err := service.ApplyCreateAndRedeemResult(ctx, successJob.ID, nil)
	if err != nil {
		t.Fatalf("ApplyCreateAndRedeemResult() success error = %v", err)
	}
	if fulfilled.Status != StatusFulfilled {
		t.Fatalf("expected fulfilled status, got %q", fulfilled.Status)
	}

	requiredJob := newJob("apply-required")
	terminal, err := service.ApplyCreateAndRedeemResult(ctx, requiredJob.ID, &proxy.APIError{StatusCode: 400, Reason: proxy.ReasonIdempotencyKeyRequired, Message: "missing key"})
	if err != nil {
		t.Fatalf("ApplyCreateAndRedeemResult() required error = %v", err)
	}
	if terminal.Status != StatusFailedTerminal {
		t.Fatalf("expected failed_terminal, got %q", terminal.Status)
	}

	conflictJob := newJob("apply-conflict")
	conflict, err := service.ApplyCreateAndRedeemResult(ctx, conflictJob.ID, &proxy.APIError{StatusCode: 409, Reason: proxy.ReasonIdempotencyKeyConflict, Message: "conflict"})
	if err != nil {
		t.Fatalf("ApplyCreateAndRedeemResult() conflict error = %v", err)
	}
	if conflict.Status != StatusFailedTerminal {
		t.Fatalf("expected conflict to be terminal, got %q", conflict.Status)
	}

	inProgressJob := newJob("apply-in-progress")
	retryable, err := service.ApplyCreateAndRedeemResult(ctx, inProgressJob.ID, &proxy.APIError{StatusCode: 409, Reason: proxy.ReasonIdempotencyInProgress, Message: "busy", RetryAfter: 3 * time.Second})
	if err != nil {
		t.Fatalf("ApplyCreateAndRedeemResult() in-progress error = %v", err)
	}
	if retryable.Status != StatusFailedRetryable {
		t.Fatalf("expected failed_retryable, got %q", retryable.Status)
	}
	if retryable.RetryCount != 1 {
		t.Fatalf("expected retry_count 1, got %d", retryable.RetryCount)
	}
	if !retryable.AvailableAt.After(time.Now().UTC()) {
		t.Fatalf("expected future available_at, got %v", retryable.AvailableAt)
	}

	backoffJob := newJob("apply-backoff")
	backoff, err := service.ApplyCreateAndRedeemResult(ctx, backoffJob.ID, &proxy.APIError{StatusCode: 409, Reason: proxy.ReasonIdempotencyRetryBackoff, Message: "retry later", RetryAfter: 5 * time.Second})
	if err != nil {
		t.Fatalf("ApplyCreateAndRedeemResult() retry-backoff error = %v", err)
	}
	if backoff.Status != StatusFailedRetryable {
		t.Fatalf("expected failed_retryable, got %q", backoff.Status)
	}

	storeDownJob := newJob("apply-store-down")
	storeDown, err := service.ApplyCreateAndRedeemResult(ctx, storeDownJob.ID, &proxy.APIError{StatusCode: 503, Reason: proxy.ReasonIdempotencyStoreDown, Message: "store unavailable"})
	if err != nil {
		t.Fatalf("ApplyCreateAndRedeemResult() store-down error = %v", err)
	}
	if storeDown.Status != StatusFailedRetryable {
		t.Fatalf("expected store-down to be retryable, got %q", storeDown.Status)
	}
}

func TestApplyBalanceRechargeResult_MapsSuccessAndRetryableErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewServiceWithDialect(database, testDialect())

	userID := createUser(t, ctx, database, "fulfillment-balance@example.com", "Balance User", "user")

	newJob := func(key string) *Job {
		t.Helper()
		job, err := service.CreateOrLoadJobByIdempotency(ctx, &CreateJobInput{
			UserID:         &userID,
			EventType:      "payment_succeeded",
			PayloadJSON:    `{"kind":"balance"}`,
			IdempotencyKey: key,
		})
		if err != nil {
			t.Fatalf("CreateOrLoadJobByIdempotency() error = %v", err)
		}
		return job
	}

	successJob := newJob("balance-success")
	fulfilled, err := service.ApplyBalanceRechargeResult(ctx, successJob.ID, nil)
	if err != nil {
		t.Fatalf("ApplyBalanceRechargeResult() success error = %v", err)
	}
	if fulfilled.Status != StatusFulfilled {
		t.Fatalf("expected fulfilled status, got %q", fulfilled.Status)
	}

	retryableJob := newJob("balance-retryable")
	retryable, err := service.ApplyBalanceRechargeResult(ctx, retryableJob.ID, &proxy.APIError{StatusCode: 503, Reason: proxy.ReasonIdempotencyStoreDown, Message: "store unavailable", RetryAfter: 2 * time.Second})
	if err != nil {
		t.Fatalf("ApplyBalanceRechargeResult() retryable error = %v", err)
	}
	if retryable.Status != StatusFailedRetryable {
		t.Fatalf("expected failed_retryable, got %q", retryable.Status)
	}
	if retryable.RetryCount != 1 {
		t.Fatalf("expected retry_count 1, got %d", retryable.RetryCount)
	}
}

func TestApplyAPIKeyCreationResult_MapsTerminalAndRetryableErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewServiceWithDialect(database, testDialect())

	userID := createUser(t, ctx, database, "fulfillment-key@example.com", "Key User", "user")

	newJob := func(key string) *Job {
		t.Helper()
		job, err := service.CreateOrLoadJobByIdempotency(ctx, &CreateJobInput{
			UserID:         &userID,
			EventType:      "payment_succeeded",
			PayloadJSON:    `{"kind":"api_key"}`,
			IdempotencyKey: key,
		})
		if err != nil {
			t.Fatalf("CreateOrLoadJobByIdempotency() error = %v", err)
		}
		return job
	}

	terminalJob := newJob("key-terminal")
	terminal, err := service.ApplyAPIKeyCreationResult(ctx, terminalJob.ID, &proxy.APIError{StatusCode: 403, Reason: "GROUP_NOT_ALLOWED", Message: "group forbidden"})
	if err != nil {
		t.Fatalf("ApplyAPIKeyCreationResult() terminal error = %v", err)
	}
	if terminal.Status != StatusFailedTerminal {
		t.Fatalf("expected failed_terminal, got %q", terminal.Status)
	}

	retryableJob := newJob("key-retryable")
	retryable, err := service.ApplyAPIKeyCreationResult(ctx, retryableJob.ID, &proxy.APIError{StatusCode: 409, Reason: proxy.ReasonIdempotencyInProgress, Message: "busy", RetryAfter: time.Second})
	if err != nil {
		t.Fatalf("ApplyAPIKeyCreationResult() retryable error = %v", err)
	}
	if retryable.Status != StatusFailedRetryable {
		t.Fatalf("expected failed_retryable, got %q", retryable.Status)
	}
	if retryable.RetryCount != 1 {
		t.Fatalf("expected retry_count 1, got %d", retryable.RetryCount)
	}
}

func TestObservabilityMetricsAndStructuredLogs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	logBuffer := new(bytes.Buffer)
	logger := slog.New(slog.NewJSONHandler(logBuffer, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service := NewServiceWithLoggerAndDialect(database, logger, testDialect())

	baseTime := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return baseTime }

	userID := createUser(t, ctx, database, "fulfillment-observability@example.com", "Observability User", "user")
	job, err := service.CreateOrLoadJobByIdempotency(ctx, &CreateJobInput{
		UserID:         &userID,
		EventType:      "payment_succeeded",
		PayloadJSON:    `{"source":"portal","kind":"api_key"}`,
		IdempotencyKey: "obs-1",
	})
	if err != nil {
		t.Fatalf("CreateOrLoadJobByIdempotency() create error = %v", err)
	}

	if _, err := service.CreateOrLoadJobByIdempotency(ctx, &CreateJobInput{
		UserID:         &userID,
		EventType:      "payment_succeeded",
		PayloadJSON:    `{"source":"portal","kind":"api_key"}`,
		IdempotencyKey: "obs-1",
	}); err != nil {
		t.Fatalf("CreateOrLoadJobByIdempotency() replay error = %v", err)
	}

	_, err = service.CreateOrLoadJobByIdempotency(ctx, &CreateJobInput{
		UserID:         &userID,
		EventType:      "payment_succeeded",
		PayloadJSON:    `{"source":"portal","kind":"balance"}`,
		IdempotencyKey: "obs-1",
	})
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected ErrIdempotencyConflict, got %v", err)
	}

	service.now = func() time.Time { return baseTime.Add(2 * time.Minute) }
	updated, err := service.ApplyAPIKeyCreationResult(ctx, job.ID, &proxy.APIError{StatusCode: 409, Reason: proxy.ReasonIdempotencyInProgress, Message: "idempotency still in progress", RetryAfter: 30 * time.Second})
	if err != nil {
		t.Fatalf("ApplyAPIKeyCreationResult() error = %v", err)
	}
	if updated.Status != StatusFailedRetryable {
		t.Fatalf("expected retryable status, got %q", updated.Status)
	}

	snapshot := service.MetricsSnapshot()
	if snapshot.JobsCreated != 1 {
		t.Fatalf("expected JobsCreated=1, got %d", snapshot.JobsCreated)
	}
	if snapshot.JobsReplayed != 1 {
		t.Fatalf("expected JobsReplayed=1, got %d", snapshot.JobsReplayed)
	}
	if snapshot.IdempotencyConflicts != 1 {
		t.Fatalf("expected IdempotencyConflicts=1, got %d", snapshot.IdempotencyConflicts)
	}
	if snapshot.EventsInserted != 2 {
		t.Fatalf("expected EventsInserted=2, got %d", snapshot.EventsInserted)
	}
	if snapshot.TransitionsTotal != 1 {
		t.Fatalf("expected TransitionsTotal=1, got %d", snapshot.TransitionsTotal)
	}
	if snapshot.RetryableFailures != 1 {
		t.Fatalf("expected RetryableFailures=1, got %d", snapshot.RetryableFailures)
	}
	if snapshot.TerminalFailures != 0 {
		t.Fatalf("expected TerminalFailures=0, got %d", snapshot.TerminalFailures)
	}
	if snapshot.LastTransitionLag != 2*time.Minute {
		t.Fatalf("expected LastTransitionLag=2m, got %v", snapshot.LastTransitionLag)
	}
	if snapshot.MaxTransitionLag != 2*time.Minute {
		t.Fatalf("expected MaxTransitionLag=2m, got %v", snapshot.MaxTransitionLag)
	}
	if snapshot.LastRetryDelay != 30*time.Second {
		t.Fatalf("expected LastRetryDelay=30s, got %v", snapshot.LastRetryDelay)
	}
	if snapshot.MaxRetryDelay != 30*time.Second {
		t.Fatalf("expected MaxRetryDelay=30s, got %v", snapshot.MaxRetryDelay)
	}

	logOutput := logBuffer.String()
	for _, needle := range []string{
		`"event":"job_created"`,
		`"event":"job_replayed"`,
		`"event":"job_create_conflict"`,
		`"event":"job_transition"`,
		`"error_class":"idempotency"`,
		`"retry_delay_ms":30000`,
	} {
		if !strings.Contains(logOutput, needle) {
			t.Fatalf("expected logs to contain %s, got %s", needle, logOutput)
		}
	}
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()
	if testDialect() == "postgres" {
		dsn := strings.TrimSpace(os.Getenv("DB_DSN"))
		if dsn == "" {
			t.Skip("DB_DSN is required when DB_DRIVER=postgres")
		}

		bootstrapDB, err := db.Open(ctx, "postgres", dsn)
		if err != nil {
			t.Fatalf("Open() bootstrap postgres error = %v", err)
		}
		defer bootstrapDB.Close()

		schemaName := testSchemaName(t)
		if _, err := bootstrapDB.ExecContext(ctx, fmt.Sprintf(`CREATE SCHEMA "%s";`, schemaName)); err != nil {
			t.Fatalf("create schema error = %v", err)
		}

		database, err := db.Open(ctx, "postgres", dsn)
		if err != nil {
			_, _ = bootstrapDB.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE;`, schemaName))
			t.Fatalf("Open() postgres error = %v", err)
		}
		t.Cleanup(func() {
			_ = database.Close()
			_, _ = bootstrapDB.ExecContext(context.Background(), fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE;`, schemaName))
		})

		if _, err := database.ExecContext(ctx, fmt.Sprintf(`SET search_path TO "%s";`, schemaName)); err != nil {
			t.Fatalf("SET search_path error = %v", err)
		}

		if err := db.ApplyMigrations(ctx, database, "postgres"); err != nil {
			t.Fatalf("ApplyMigrations() postgres error = %v", err)
		}

		return database
	}

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

func createUser(t *testing.T, ctx context.Context, database *sql.DB, email, name, role string) int64 {
	t.Helper()

	id, err := db.InsertID(ctx, testDialect(), database, `INSERT INTO als_users(email, name, role) VALUES (?, ?, ?);`, "id", email, name, role)
	if err != nil {
		t.Fatalf("insert user error = %v", err)
	}

	return id
}

func createSubscription(t *testing.T, ctx context.Context, database *sql.DB, userID int64, tierCode, tierName string) int64 {
	t.Helper()

	tierID, err := db.InsertID(ctx, testDialect(), database, `INSERT INTO als_tiers(code, name) VALUES (?, ?);`, "id", tierCode, tierName)
	if err != nil {
		t.Fatalf("insert tier error = %v", err)
	}

	now := time.Now().UTC()
	subscriptionID, err := db.InsertID(ctx, testDialect(), database, `
		INSERT INTO als_subscriptions(user_id, tier_id, status, started_at, created_at, updated_at)
		VALUES (?, ?, 'active', ?, ?, ?);
	`, "id", userID, tierID, now, now, now)
	if err != nil {
		t.Fatalf("insert subscription error = %v", err)
	}

	return subscriptionID
}

func assertCount(t *testing.T, ctx context.Context, database *sql.DB, query string, arg any, expected int64) {
	t.Helper()

	var (
		count int64
		err   error
	)
	query = db.Rebind(testDialect(), query)
	if arg == nil {
		err = database.QueryRowContext(ctx, query).Scan(&count)
	} else {
		err = database.QueryRowContext(ctx, query, arg).Scan(&count)
	}
	if err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != expected {
		t.Fatalf("expected count %d, got %d for query %q", expected, count, query)
	}
}

func ptr[T any](value T) *T {
	return &value
}

func testDialect() string {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("DB_DRIVER")), "postgres") {
		return "postgres"
	}

	return "sqlite"
}

func testSchemaName(t *testing.T) string {
	t.Helper()

	name := strings.ToLower(t.Name())
	replacer := strings.NewReplacer("/", "_", " ", "_", "-", "_", ":", "_", ".", "_")
	name = replacer.Replace(name)
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '_':
			return r
		default:
			return '_'
		}
	}, name)
	name = strings.Trim(name, "_")
	if name == "" {
		name = "fulfillment_test"
	}

	return fmt.Sprintf("%s_%d", name, time.Now().UTC().UnixNano())
}
