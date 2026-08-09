package fulfillment

import (
	"context"
	"sync"
	"testing"
	"time"

	"ai-api-portal/backend/internal/proxy"
)

// makeRetryableJob creates a job and drives it into failed_retryable with the
// given backoff availability and retry count, returning the reloaded job.
func makeRetryableJob(t *testing.T, ctx context.Context, svc *Service, userID int64, idemKey string, availableAt time.Time, retryCount int) *Job {
	t.Helper()
	job, err := svc.CreateOrLoadJobByIdempotency(ctx, &CreateJobInput{
		UserID:         &userID,
		EventType:      "payment_succeeded",
		PayloadJSON:    `{"source":"stripe"}`,
		IdempotencyKey: idemKey,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	rc := retryCount
	if _, err := svc.TransitionJob(ctx, job.ID, &TransitionInput{
		Status:       StatusFailedRetryable,
		EventType:    "force_failed_retryable",
		AvailableAt:  &availableAt,
		RetryCount:   &rc,
		EventPayload: ptr(`{"reason":"test"}`),
	}); err != nil {
		t.Fatalf("transition to failed_retryable: %v", err)
	}
	got, err := svc.GetJobByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("reload job: %v", err)
	}
	return got
}

func TestWorker_RunOnce_ClaimsAndProcessesDueJob(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := setupTestDB(t)
	svc := NewServiceWithDialect(database, testDialect())
	userID := createUser(t, ctx, database, "worker-due@example.com", "Worker Due", "user")

	past := time.Now().UTC().Add(-1 * time.Minute)
	job := makeRetryableJob(t, ctx, svc, userID, "worker-due-1", past, 0)

	var mu sync.Mutex
	var seen []int64
	worker, err := New(svc, func(ctx context.Context, j *Job) error {
		mu.Lock()
		seen = append(seen, j.ID)
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("New worker: %v", err)
	}
	worker.runOnce(ctx)

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 1 || seen[0] != job.ID {
		t.Fatalf("expected process to see job %d once, got %v", job.ID, seen)
	}
	reloaded, err := svc.GetJobByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != StatusPaidUnfulfilled {
		t.Fatalf("expected claimed job status %q, got %q", StatusPaidUnfulfilled, reloaded.Status)
	}
}

func TestWorker_RunOnce_SkipsNotYetDueJob(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := setupTestDB(t)
	svc := NewServiceWithDialect(database, testDialect())
	userID := createUser(t, ctx, database, "worker-future@example.com", "Worker Future", "user")

	future := time.Now().UTC().Add(30 * time.Minute)
	makeRetryableJob(t, ctx, svc, userID, "worker-future-1", future, 0)

	var called int
	worker, _ := New(svc, func(ctx context.Context, j *Job) error {
		called++
		return nil
	})
	worker.runOnce(ctx)
	if called != 0 {
		t.Fatalf("expected no processing before available_at, got %d", called)
	}
}

func TestWorker_RunOnce_DoesNotReprocessAlreadyClaimed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := setupTestDB(t)
	svc := NewServiceWithDialect(database, testDialect())
	userID := createUser(t, ctx, database, "worker-taken@example.com", "Worker Taken", "user")

	past := time.Now().UTC().Add(-1 * time.Minute)
	job := makeRetryableJob(t, ctx, svc, userID, "worker-taken-1", past, 0)

	// A prior claimant (e.g. another instance) already took it.
	if claimed, err := svc.ClaimRetryableJob(ctx, job.ID); err != nil || !claimed {
		t.Fatalf("pre-claim: claimed=%v err=%v", claimed, err)
	}

	var called int
	worker, _ := New(svc, func(ctx context.Context, j *Job) error {
		called++
		return nil
	})
	// ListRetryDueJobs excludes the now-paid_unfulfilled job, so process is not called.
	worker.runOnce(ctx)
	if called != 0 {
		t.Fatalf("expected no processing for already-claimed job, got %d", called)
	}
}

func TestWorker_ListRetryDueJobs_ExcludesExhaustedRetries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := setupTestDB(t)
	svc := NewServiceWithDialect(database, testDialect())
	svc.SetMaxRetries(3)
	userID := createUser(t, ctx, database, "worker-cap@example.com", "Worker Cap", "user")

	past := time.Now().UTC().Add(-1 * time.Minute)
	makeRetryableJob(t, ctx, svc, userID, "worker-cap-1", past, 3) // retry_count at the cap

	due, err := svc.ListRetryDueJobs(ctx, time.Now().UTC(), 25)
	if err != nil {
		t.Fatalf("ListRetryDueJobs: %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("expected exhausted-retry job to be excluded, got %d", len(due))
	}
}

func TestApplyProxyResult_RetryCapForcesTerminal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := setupTestDB(t)
	svc := NewServiceWithDialect(database, testDialect())
	svc.SetMaxRetries(2)
	userID := createUser(t, ctx, database, "worker-retrycap@example.com", "Retry Cap", "user")

	job, err := svc.CreateOrLoadJobByIdempotency(ctx, &CreateJobInput{
		UserID:         &userID,
		EventType:      "payment_succeeded",
		PayloadJSON:    `{"source":"stripe"}`,
		IdempotencyKey: "retrycap-1",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	rc := 2 // already at the cap
	if _, err := svc.TransitionJob(ctx, job.ID, &TransitionInput{
		Status:       StatusFailedRetryable,
		EventType:    "seed_failed_retryable",
		RetryCount:   &rc,
		EventPayload: ptr(`{}`),
	}); err != nil {
		t.Fatalf("seed transition: %v", err)
	}

	retryableErr := &proxy.APIError{StatusCode: 503}
	if !retryableErr.IsRetryable() {
		t.Fatalf("seed error must be retryable")
	}
	updated, err := svc.ApplyPackageFulfillmentResult(ctx, job.ID, retryableErr)
	if err != nil {
		t.Fatalf("ApplyPackageFulfillmentResult: %v", err)
	}
	if updated.Status != StatusFailedTerminal {
		t.Fatalf("expected terminal after exhausting retries, got %q (err=%v)", updated.Status, updated.ErrorMessage)
	}
}

func TestClaimRetryableJob_SingleWinnerAcrossInstances(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := setupTestDB(t)
	svc1 := NewServiceWithDialect(database, testDialect())
	svc2 := NewServiceWithDialect(database, testDialect())
	userID := createUser(t, ctx, database, "worker-cas@example.com", "CAS", "user")

	past := time.Now().UTC().Add(-1 * time.Minute)
	job := makeRetryableJob(t, ctx, svc1, userID, "worker-cas-1", past, 0)

	w1, err := svc1.ClaimRetryableJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("svc1 claim: %v", err)
	}
	w2, err := svc2.ClaimRetryableJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("svc2 claim: %v", err)
	}
	if w1 == w2 {
		t.Fatalf("expected exactly one claim winner, both=%v", w1)
	}
}
