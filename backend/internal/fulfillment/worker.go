package fulfillment

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// ProcessFunc is invoked by the Worker for each job it wins a claim on. It must
// be idempotent (the same job may be processed more than once across instances
// or replays) and must transition the job to its next status itself — typically
// by delegating to the fulfillment Service's Apply*/retry helpers.
type ProcessFunc func(ctx context.Context, job *Job) error

// Worker polls als_fulfillment_jobs for failed_retryable rows whose backoff has
// elapsed and drives them through ProcessFunc. The Service's MaxRetries cap
// bounds the work, and an atomic CAS claim (failed_retryable -> paid_unfulfilled)
// means multiple instances do not double-process the same job. Even if two
// instances briefly race, upstream Idempotency-Keys keep grants safe.
type Worker struct {
	svc       *Service
	process   ProcessFunc
	interval  time.Duration
	batchSize int
	logger    *slog.Logger
	now       func() time.Time
}

// Option configures a Worker.
type Option func(*Worker)

// WithInterval overrides the poll interval (default 10s).
func WithInterval(d time.Duration) Option {
	return func(w *Worker) {
		if d > 0 {
			w.interval = d
		}
	}
}

// WithBatchSize overrides the max jobs processed per tick (default 25).
func WithBatchSize(n int) Option {
	return func(w *Worker) {
		if n > 0 {
			w.batchSize = n
		}
	}
}

// WithLogger overrides the logger.
func WithLogger(logger *slog.Logger) Option {
	return func(w *Worker) {
		if logger != nil {
			w.logger = logger.With("component", "fulfillment_worker")
		}
	}
}

// WithClock overrides the clock used to decide "now" (mainly for tests).
func WithClock(now func() time.Time) Option {
	return func(w *Worker) {
		if now != nil {
			w.now = func() time.Time { return now().UTC() }
		}
	}
}

// New builds a Worker. svc and process are required.
func New(svc *Service, process ProcessFunc, opts ...Option) (*Worker, error) {
	if svc == nil {
		return nil, errors.New("fulfillment service is required")
	}
	if process == nil {
		return nil, errors.New("process function is required")
	}
	w := &Worker{
		svc:       svc,
		process:   process,
		interval:  10 * time.Second,
		batchSize: 25,
		logger:    slog.Default().With("component", "fulfillment_worker"),
		now:       func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(w)
	}
	return w, nil
}

// Start runs the poll loop until ctx is cancelled. Run it in its own goroutine.
// Each tick processes up to batchSize due jobs.
func (w *Worker) Start(ctx context.Context) {
	if w == nil || w.svc == nil || w.process == nil {
		return
	}
	w.logger.LogAttrs(ctx, slog.LevelInfo, "fulfillment worker started",
		slog.Duration("interval", w.interval), slog.Int("batch_size", w.batchSize))
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.logger.LogAttrs(context.Background(), slog.LevelInfo, "fulfillment worker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

// runOnce processes a single batch. Split out so tests can drive one tick
// synchronously without waiting on the ticker.
func (w *Worker) runOnce(ctx context.Context) {
	now := w.now()
	jobs, err := w.svc.ListRetryDueJobs(ctx, now, w.batchSize)
	if err != nil {
		w.logger.LogAttrs(ctx, slog.LevelWarn, "fulfillment worker: list due jobs failed",
			slog.String("error", err.Error()))
		return
	}
	for _, job := range jobs {
		if ctx.Err() != nil {
			return
		}
		claimed, err := w.svc.ClaimRetryableJob(ctx, job.ID)
		if err != nil {
			w.logger.LogAttrs(ctx, slog.LevelWarn, "fulfillment worker: claim failed",
				slog.Int64("job_id", job.ID), slog.String("error", err.Error()))
			continue
		}
		if !claimed {
			continue // lost the race to another instance / concurrent replayer
		}
		w.logger.LogAttrs(ctx, slog.LevelInfo, "fulfillment worker: processing job",
			slog.Int64("job_id", job.ID), slog.Int("retry_count", job.RetryCount))
		if err := w.process(ctx, job); err != nil {
			w.logger.LogAttrs(ctx, slog.LevelWarn, "fulfillment worker: process failed",
				slog.Int64("job_id", job.ID), slog.String("error", err.Error()))
		}
	}
}
