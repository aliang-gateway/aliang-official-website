package fulfillment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/proxy"
)

const (
	StatusPaidUnfulfilled = "paid_unfulfilled"
	StatusFulfilled       = "fulfilled"
	StatusFailedRetryable = "failed_retryable"
	StatusFailedTerminal  = "failed_terminal"
)

const (
	EventTypeJobCreated = "job_created"
)

type Service struct {
	db         *sql.DB
	logger     *slog.Logger
	metrics    *serviceMetrics
	now        func() time.Time
	sqlDialect string
}

type MetricsSnapshot struct {
	JobsCreated          int64
	JobsReplayed         int64
	IdempotencyConflicts int64
	EventsInserted       int64
	TransitionsTotal     int64
	FulfilledTotal       int64
	RetryableFailures    int64
	TerminalFailures     int64
	LastTransitionLag    time.Duration
	MaxTransitionLag     time.Duration
	LastRetryDelay       time.Duration
	MaxRetryDelay        time.Duration
}

type serviceMetrics struct {
	jobsCreated          atomic.Int64
	jobsReplayed         atomic.Int64
	idempotencyConflicts atomic.Int64
	eventsInserted       atomic.Int64
	transitionsTotal     atomic.Int64
	fulfilledTotal       atomic.Int64
	retryableFailures    atomic.Int64
	terminalFailures     atomic.Int64
	lastTransitionLagNs  atomic.Int64
	maxTransitionLagNs   atomic.Int64
	lastRetryDelayNs     atomic.Int64
	maxRetryDelayNs      atomic.Int64
}

type Job struct {
	ID             int64
	UserID         *int64
	SubscriptionID *int64
	EventType      string
	Status         string
	PayloadJSON    string
	ErrorMessage   *string
	AvailableAt    time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	RetryCount     int
	IdempotencyKey *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Event struct {
	ID               int64
	FulfillmentJobID int64
	EventType        string
	PayloadJSON      *string
	CreatedAt        time.Time
}

type CreateJobInput struct {
	UserID         *int64
	SubscriptionID *int64
	EventType      string
	PayloadJSON    string
	IdempotencyKey string
	AvailableAt    *time.Time
}

type TransitionInput struct {
	Status       string
	ErrorMessage *string
	EventType    string
	EventPayload *string
	AvailableAt  *time.Time
	StartedAt    *time.Time
	FinishedAt   *time.Time
	RetryCount   *int
}

func NewService(database *sql.DB) *Service {
	return NewServiceWithLoggerAndDialect(database, nil, "sqlite")
}

func NewServiceWithLogger(database *sql.DB, logger *slog.Logger) *Service {
	return NewServiceWithLoggerAndDialect(database, logger, "sqlite")
}

func NewServiceWithDialect(database *sql.DB, sqlDialect string) *Service {
	return NewServiceWithLoggerAndDialect(database, nil, sqlDialect)
}

func NewServiceWithLoggerAndDialect(database *sql.DB, logger *slog.Logger, sqlDialect string) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		db:         database,
		logger:     logger.With("component", "fulfillment"),
		metrics:    &serviceMetrics{},
		sqlDialect: sqlDialect,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Service) MetricsSnapshot() MetricsSnapshot {
	if s == nil || s.metrics == nil {
		return MetricsSnapshot{}
	}
	return MetricsSnapshot{
		JobsCreated:          s.metrics.jobsCreated.Load(),
		JobsReplayed:         s.metrics.jobsReplayed.Load(),
		IdempotencyConflicts: s.metrics.idempotencyConflicts.Load(),
		EventsInserted:       s.metrics.eventsInserted.Load(),
		TransitionsTotal:     s.metrics.transitionsTotal.Load(),
		FulfilledTotal:       s.metrics.fulfilledTotal.Load(),
		RetryableFailures:    s.metrics.retryableFailures.Load(),
		TerminalFailures:     s.metrics.terminalFailures.Load(),
		LastTransitionLag:    time.Duration(s.metrics.lastTransitionLagNs.Load()),
		MaxTransitionLag:     time.Duration(s.metrics.maxTransitionLagNs.Load()),
		LastRetryDelay:       time.Duration(s.metrics.lastRetryDelayNs.Load()),
		MaxRetryDelay:        time.Duration(s.metrics.maxRetryDelayNs.Load()),
	}
}

func (s *Service) CreateOrLoadJobByIdempotency(ctx context.Context, input *CreateJobInput) (*Job, error) {
	if input == nil {
		return nil, errors.New("create job input is required")
	}

	eventType := strings.TrimSpace(input.EventType)
	if eventType == "" {
		return nil, errors.New("event type is required")
	}

	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey == "" {
		return nil, errors.New("idempotency key is required")
	}

	payload := strings.TrimSpace(input.PayloadJSON)
	now := s.currentTime()
	availableAt := now
	if input.AvailableAt != nil {
		availableAt = input.AvailableAt.UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create fulfillment job tx: %w", err)
	}

	job, exists, err := s.getJobByIdempotencyKeyTx(ctx, tx, idempotencyKey)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if exists {
		if !isIdempotentCreateMatch(job, input, payload) {
			s.recordIdempotencyConflict()
			s.logWarn(ctx, "fulfillment job idempotency conflict", slog.String("event", "job_create_conflict"), slog.String("idempotency_key", idempotencyKey), slog.String("event_type", eventType), slog.Int64("existing_job_id", job.ID))
			_ = tx.Rollback()
			return nil, ErrIdempotencyConflict
		}
		s.recordReplay()
		s.logInfo(ctx, "fulfillment job replayed", slog.String("event", "job_replayed"), slog.Int64("job_id", job.ID), slog.String("idempotency_key", idempotencyKey), slog.String("status", job.Status), slog.Int("retry_count", job.RetryCount))
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit create fulfillment job tx (existing): %w", err)
		}
		return job, nil
	}

	jobID, err := db.InsertID(ctx, s.sqlDialect, tx, `
		INSERT INTO als_fulfillment_jobs(
			user_id,
			subscription_id,
			event_type,
			status,
			payload_json,
			available_at,
			idempotency_key,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "id", input.UserID, input.SubscriptionID, eventType, StatusPaidUnfulfilled, payload, availableAt, idempotencyKey, now, now)
	if err != nil {
		if isUniqueConstraintError(err) {
			existing, found, getErr := s.getJobByIdempotencyKeyTx(ctx, tx, idempotencyKey)
			if getErr != nil {
				_ = tx.Rollback()
				return nil, getErr
			}
			if !found {
				_ = tx.Rollback()
				return nil, fmt.Errorf("read existing fulfillment job after idempotency conflict: %w", err)
			}
			if !isIdempotentCreateMatch(existing, input, payload) {
				s.recordIdempotencyConflict()
				s.logWarn(ctx, "fulfillment job idempotency conflict after unique violation", slog.String("event", "job_create_conflict"), slog.String("idempotency_key", idempotencyKey), slog.String("event_type", eventType), slog.Int64("existing_job_id", existing.ID))
				_ = tx.Rollback()
				return nil, ErrIdempotencyConflict
			}
			s.recordReplay()
			s.logInfo(ctx, "fulfillment job replayed after unique violation", slog.String("event", "job_replayed"), slog.Int64("job_id", existing.ID), slog.String("idempotency_key", idempotencyKey), slog.String("status", existing.Status), slog.Int("retry_count", existing.RetryCount))
			if commitErr := tx.Commit(); commitErr != nil {
				return nil, fmt.Errorf("commit create fulfillment job tx (idempotent existing): %w", commitErr)
			}
			return existing, nil
		}
		_ = tx.Rollback()
		return nil, fmt.Errorf("insert fulfillment job: %w", err)
	}

	if err := insertEventTx(ctx, tx, s.sqlDialect, jobID, EventTypeJobCreated, nullableString(payload), now); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	s.recordEventInserted()

	job, err = s.getJobByIDTx(ctx, tx, jobID)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit create fulfillment job tx: %w", err)
	}
	s.recordJobCreated()
	s.logInfo(ctx, "fulfillment job created", slog.String("event", "job_created"), slog.Int64("job_id", job.ID), slog.String("idempotency_key", idempotencyKey), slog.String("event_type", job.EventType), slog.String("status", job.Status), slog.Int("retry_count", job.RetryCount), slog.Time("available_at", job.AvailableAt), slog.Bool("has_user_id", job.UserID != nil), slog.Bool("has_subscription_id", job.SubscriptionID != nil))

	return job, nil
}

func (s *Service) GetJobByID(ctx context.Context, id int64) (*Job, error) {
	if id <= 0 {
		return nil, errors.New("job id must be positive")
	}

	job, err := s.getJobByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return job, nil
}

func (s *Service) ListEventsByJobID(ctx context.Context, jobID int64) ([]Event, error) {
	if jobID <= 0 {
		return nil, errors.New("job id must be positive")
	}

	rows, err := s.db.QueryContext(ctx, db.Rebind(s.sqlDialect, `
		SELECT id, fulfillment_job_id, event_type, payload_json, created_at
		FROM als_fulfillment_events
		WHERE fulfillment_job_id = ?
		ORDER BY id ASC;
	`), jobID)
	if err != nil {
		return nil, fmt.Errorf("query fulfillment events: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		var (
			event       Event
			payloadJSON sql.NullString
		)
		if err := rows.Scan(&event.ID, &event.FulfillmentJobID, &event.EventType, &payloadJSON, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan fulfillment event: %w", err)
		}
		if payloadJSON.Valid {
			v := payloadJSON.String
			event.PayloadJSON = &v
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate fulfillment events: %w", err)
	}

	return events, nil
}

func (s *Service) TransitionJob(ctx context.Context, jobID int64, input *TransitionInput) (*Job, error) {
	if jobID <= 0 {
		return nil, errors.New("job id must be positive")
	}
	if input == nil {
		return nil, errors.New("transition input is required")
	}

	nextStatus := strings.TrimSpace(input.Status)
	if !isValidStatus(nextStatus) {
		return nil, errors.New("invalid fulfillment status")
	}

	eventType := strings.TrimSpace(input.EventType)
	if eventType == "" {
		return nil, errors.New("event type is required")
	}

	now := s.currentTime()
	errorMessage := normalizeErrorMessage(input.ErrorMessage)
	availableAt := resolveUTCOrNow(input.AvailableAt, now)
	startedAt := resolveOptionalUTC(input.StartedAt)
	finishedAt := resolveOptionalUTC(input.FinishedAt)
	retryCount := 0
	if input.RetryCount != nil {
		retryCount = *input.RetryCount
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transition fulfillment job tx: %w", err)
	}

	job, err := s.getJobByIDTx(ctx, tx, jobID)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	if !canTransition(job.Status, nextStatus) {
		s.logWarn(ctx, "fulfillment transition rejected", slog.String("event", "transition_rejected"), slog.Int64("job_id", jobID), slog.String("from_status", job.Status), slog.String("to_status", nextStatus), slog.String("event_type", eventType))
		_ = tx.Rollback()
		return nil, ErrInvalidTransition
	}

	if _, err := tx.ExecContext(ctx, db.Rebind(s.sqlDialect, `
		UPDATE als_fulfillment_jobs
		SET
			status = ?,
			error_message = ?,
			available_at = ?,
			started_at = ?,
			finished_at = ?,
			retry_count = ?,
			updated_at = ?
		WHERE id = ?;
	`), nextStatus, errorMessage, availableAt, startedAt, finishedAt, retryCount, now, jobID); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("update fulfillment job transition: %w", err)
	}

	if err := insertEventTx(ctx, tx, s.sqlDialect, jobID, eventType, input.EventPayload, now); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	s.recordEventInserted()

	updated, err := s.getJobByIDTx(ctx, tx, jobID)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transition fulfillment job tx: %w", err)
	}

	lag := now.Sub(job.CreatedAt)
	s.recordTransitionOutcome(updated.Status, lag, retryDelayForStatus(updated.Status, now, updated.AvailableAt))
	s.logTransition(ctx, updated, job.Status, eventType, lag)

	return updated, nil
}

func (s *Service) ApplyCreateAndRedeemResult(ctx context.Context, jobID int64, resultErr error) (*Job, error) {
	job, err := s.GetJobByID(ctx, jobID)
	if err != nil {
		return nil, err
	}

	now := s.currentTime()
	if resultErr == nil {
		return s.TransitionJob(ctx, jobID, &TransitionInput{
			Status:       StatusFulfilled,
			EventType:    "sub2api_create_and_redeem_succeeded",
			StartedAt:    &now,
			FinishedAt:   &now,
			RetryCount:   &job.RetryCount,
			EventPayload: nullableString(`{"outcome":"fulfilled"}`),
		})
	}

	errorMessage := resultErr.Error()
	retryCount := job.RetryCount
	status := StatusFailedTerminal
	eventType := "sub2api_create_and_redeem_failed_terminal"
	availableAt := now
	startedAt := now
	finishedAt := now

	var apiErr *proxy.APIError
	if errors.As(resultErr, &apiErr) {
		if apiErr.IsRetryable() {
			status = StatusFailedRetryable
			eventType = "sub2api_create_and_redeem_failed_retryable"
			retryCount++
			finishedAt = time.Time{}
			if apiErr.RetryAfter > 0 {
				availableAt = now.Add(apiErr.RetryAfter)
			} else {
				availableAt = now.Add(time.Minute)
			}
		}
	}

	transition := &TransitionInput{
		Status:       status,
		ErrorMessage: &errorMessage,
		EventType:    eventType,
		StartedAt:    &startedAt,
		RetryCount:   &retryCount,
		EventPayload: nullableString(fmt.Sprintf(`{"error":%q}`, strings.TrimSpace(errorMessage))),
	}
	if status == StatusFailedRetryable {
		transition.AvailableAt = &availableAt
	} else {
		transition.FinishedAt = &finishedAt
	}

	return s.TransitionJob(ctx, jobID, transition)
}

func (s *Service) ApplyBalanceRechargeResult(ctx context.Context, jobID int64, resultErr error) (*Job, error) {
	return s.applyProxyResult(ctx, jobID, resultErr, "sub2api_balance_recharge")
}

func (s *Service) ApplyAPIKeyCreationResult(ctx context.Context, jobID int64, resultErr error) (*Job, error) {
	return s.applyProxyResult(ctx, jobID, resultErr, "sub2api_api_key_create")
}

func (s *Service) applyProxyResult(ctx context.Context, jobID int64, resultErr error, eventPrefix string) (*Job, error) {
	job, err := s.GetJobByID(ctx, jobID)
	if err != nil {
		return nil, err
	}

	now := s.currentTime()
	if resultErr == nil {
		return s.TransitionJob(ctx, jobID, &TransitionInput{
			Status:       StatusFulfilled,
			EventType:    eventPrefix + "_succeeded",
			StartedAt:    &now,
			FinishedAt:   &now,
			RetryCount:   &job.RetryCount,
			EventPayload: nullableString(`{"outcome":"fulfilled"}`),
		})
	}

	errorMessage := resultErr.Error()
	retryCount := job.RetryCount
	status := StatusFailedTerminal
	eventType := eventPrefix + "_failed_terminal"
	availableAt := now
	startedAt := now
	finishedAt := now

	var apiErr *proxy.APIError
	if errors.As(resultErr, &apiErr) {
		if apiErr.IsRetryable() {
			status = StatusFailedRetryable
			eventType = eventPrefix + "_failed_retryable"
			retryCount++
			finishedAt = time.Time{}
			if apiErr.RetryAfter > 0 {
				availableAt = now.Add(apiErr.RetryAfter)
			} else {
				availableAt = now.Add(time.Minute)
			}
		}
	}

	transition := &TransitionInput{
		Status:       status,
		ErrorMessage: &errorMessage,
		EventType:    eventType,
		StartedAt:    &startedAt,
		RetryCount:   &retryCount,
		EventPayload: nullableString(fmt.Sprintf(`{"error":%q}`, strings.TrimSpace(errorMessage))),
	}
	if status == StatusFailedRetryable {
		transition.AvailableAt = &availableAt
	} else {
		transition.FinishedAt = &finishedAt
	}

	return s.TransitionJob(ctx, jobID, transition)
}

func (s *Service) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) logInfo(ctx context.Context, msg string, attrs ...slog.Attr) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.LogAttrs(ctx, slog.LevelInfo, msg, attrs...)
}

func (s *Service) logWarn(ctx context.Context, msg string, attrs ...slog.Attr) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.LogAttrs(ctx, slog.LevelWarn, msg, attrs...)
}

func (s *Service) logTransition(ctx context.Context, updated *Job, fromStatus, eventType string, lag time.Duration) {
	if updated == nil {
		return
	}
	attrs := []slog.Attr{
		slog.String("event", "job_transition"),
		slog.Int64("job_id", updated.ID),
		slog.String("from_status", fromStatus),
		slog.String("to_status", updated.Status),
		slog.String("event_type", eventType),
		slog.Int("retry_count", updated.RetryCount),
		slog.Int64("lag_ms", lag.Milliseconds()),
		slog.Time("available_at", updated.AvailableAt),
	}
	if updated.ErrorMessage != nil {
		attrs = append(attrs, slog.Bool("has_error", true), classifyErrorAttr(*updated.ErrorMessage))
	} else {
		attrs = append(attrs, slog.Bool("has_error", false))
	}
	if retryDelay := retryDelayForStatus(updated.Status, s.currentTime(), updated.AvailableAt); retryDelay > 0 {
		attrs = append(attrs, slog.Int64("retry_delay_ms", retryDelay.Milliseconds()))
	}
	s.logInfo(ctx, "fulfillment job transitioned", attrs...)
}

func classifyErrorAttr(message string) slog.Attr {
	message = strings.TrimSpace(message)
	if message == "" {
		return slog.String("error_class", "none")
	}
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "idempotency"):
		return slog.String("error_class", "idempotency")
	case strings.Contains(lower, "group"):
		return slog.String("error_class", "group")
	case strings.Contains(lower, "token") || strings.Contains(lower, "auth"):
		return slog.String("error_class", "auth")
	default:
		return slog.String("error_class", "generic")
	}
}

func retryDelayForStatus(status string, now, availableAt time.Time) time.Duration {
	if status != StatusFailedRetryable {
		return 0
	}
	if availableAt.Before(now) || availableAt.Equal(now) {
		return 0
	}
	return availableAt.Sub(now)
}

func (s *Service) recordJobCreated() {
	if s == nil || s.metrics == nil {
		return
	}
	s.metrics.jobsCreated.Add(1)
}

func (s *Service) recordReplay() {
	if s == nil || s.metrics == nil {
		return
	}
	s.metrics.jobsReplayed.Add(1)
}

func (s *Service) recordIdempotencyConflict() {
	if s == nil || s.metrics == nil {
		return
	}
	s.metrics.idempotencyConflicts.Add(1)
}

func (s *Service) recordEventInserted() {
	if s == nil || s.metrics == nil {
		return
	}
	s.metrics.eventsInserted.Add(1)
}

func (s *Service) recordTransitionOutcome(status string, lag, retryDelay time.Duration) {
	if s == nil || s.metrics == nil {
		return
	}
	s.metrics.transitionsTotal.Add(1)
	storeMax(&s.metrics.maxTransitionLagNs, lag)
	s.metrics.lastTransitionLagNs.Store(lag.Nanoseconds())
	if retryDelay > 0 {
		s.metrics.lastRetryDelayNs.Store(retryDelay.Nanoseconds())
		storeMax(&s.metrics.maxRetryDelayNs, retryDelay)
	}
	switch status {
	case StatusFulfilled:
		s.metrics.fulfilledTotal.Add(1)
	case StatusFailedRetryable:
		s.metrics.retryableFailures.Add(1)
	case StatusFailedTerminal:
		s.metrics.terminalFailures.Add(1)
	}
}

func storeMax(target *atomic.Int64, value time.Duration) {
	newValue := value.Nanoseconds()
	for {
		current := target.Load()
		if newValue <= current {
			return
		}
		if target.CompareAndSwap(current, newValue) {
			return
		}
	}
}

func (s *Service) getJobByID(ctx context.Context, id int64) (*Job, error) {
	job, err := s.scanJob(
		s.db.QueryRowContext(ctx, db.Rebind(s.sqlDialect, `
			SELECT
				id,
				user_id,
				subscription_id,
				event_type,
				status,
				payload_json,
				error_message,
				available_at,
				started_at,
				finished_at,
				retry_count,
				idempotency_key,
				created_at,
				updated_at
			FROM als_fulfillment_jobs
			WHERE id = ?
			LIMIT 1;
		`), id),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query fulfillment job by id: %w", err)
	}
	return job, nil
}

func (s *Service) getJobByIDTx(ctx context.Context, tx *sql.Tx, id int64) (*Job, error) {
	job, err := s.scanJob(
		tx.QueryRowContext(ctx, db.Rebind(s.sqlDialect, `
			SELECT
				id,
				user_id,
				subscription_id,
				event_type,
				status,
				payload_json,
				error_message,
				available_at,
				started_at,
				finished_at,
				retry_count,
				idempotency_key,
				created_at,
				updated_at
			FROM als_fulfillment_jobs
			WHERE id = ?
			LIMIT 1;
		`), id),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query fulfillment job by id in tx: %w", err)
	}
	return job, nil
}

func (s *Service) getJobByIdempotencyKeyTx(ctx context.Context, tx *sql.Tx, idempotencyKey string) (*Job, bool, error) {
	job, err := s.scanJob(
		tx.QueryRowContext(ctx, db.Rebind(s.sqlDialect, `
			SELECT
				id,
				user_id,
				subscription_id,
				event_type,
				status,
				payload_json,
				error_message,
				available_at,
				started_at,
				finished_at,
				retry_count,
				idempotency_key,
				created_at,
				updated_at
			FROM als_fulfillment_jobs
			WHERE idempotency_key = ?
			LIMIT 1;
		`), idempotencyKey),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("query fulfillment job by idempotency key: %w", err)
	}
	return job, true, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (s *Service) scanJob(scanner rowScanner) (*Job, error) {
	var (
		job            Job
		userID         sql.NullInt64
		subscriptionID sql.NullInt64
		errorMessage   sql.NullString
		startedAt      sql.NullTime
		finishedAt     sql.NullTime
		idempotencyKey sql.NullString
		payloadJSON    sql.NullString
	)

	err := scanner.Scan(
		&job.ID,
		&userID,
		&subscriptionID,
		&job.EventType,
		&job.Status,
		&payloadJSON,
		&errorMessage,
		&job.AvailableAt,
		&startedAt,
		&finishedAt,
		&job.RetryCount,
		&idempotencyKey,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if userID.Valid {
		v := userID.Int64
		job.UserID = &v
	}
	if subscriptionID.Valid {
		v := subscriptionID.Int64
		job.SubscriptionID = &v
	}
	if payloadJSON.Valid {
		job.PayloadJSON = payloadJSON.String
	}
	if errorMessage.Valid {
		v := errorMessage.String
		job.ErrorMessage = &v
	}
	if startedAt.Valid {
		v := startedAt.Time
		job.StartedAt = &v
	}
	if finishedAt.Valid {
		v := finishedAt.Time
		job.FinishedAt = &v
	}
	if idempotencyKey.Valid {
		v := idempotencyKey.String
		job.IdempotencyKey = &v
	}

	return &job, nil
}

func insertEventTx(ctx context.Context, tx *sql.Tx, sqlDialect string, jobID int64, eventType string, payloadJSON *string, createdAt time.Time) error {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return errors.New("event type is required")
	}

	if _, err := tx.ExecContext(ctx, db.Rebind(sqlDialect, `
		INSERT INTO als_fulfillment_events(
			fulfillment_job_id,
			event_type,
			payload_json,
			created_at
		)
		VALUES (?, ?, ?, ?);
	`), jobID, eventType, payloadJSON, createdAt); err != nil {
		return fmt.Errorf("insert fulfillment event: %w", err)
	}

	return nil
}

func canTransition(currentStatus, nextStatus string) bool {
	currentStatus = strings.TrimSpace(currentStatus)
	nextStatus = strings.TrimSpace(nextStatus)

	if currentStatus == nextStatus {
		return true
	}

	switch currentStatus {
	case StatusPaidUnfulfilled:
		return nextStatus == StatusFulfilled || nextStatus == StatusFailedRetryable || nextStatus == StatusFailedTerminal
	case StatusFailedRetryable:
		return nextStatus == StatusPaidUnfulfilled || nextStatus == StatusFailedTerminal || nextStatus == StatusFulfilled
	case StatusFulfilled, StatusFailedTerminal:
		return false
	default:
		return false
	}
}

func isValidStatus(status string) bool {
	status = strings.TrimSpace(status)
	switch status {
	case StatusPaidUnfulfilled, StatusFulfilled, StatusFailedRetryable, StatusFailedTerminal:
		return true
	default:
		return false
	}
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "unique constraint") ||
		strings.Contains(errText, "is not unique") ||
		strings.Contains(errText, "duplicate key")
}

func nullableString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	v := trimmed
	return &v
}

func normalizeErrorMessage(errorMessage *string) *string {
	if errorMessage == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*errorMessage)
	if trimmed == "" {
		return nil
	}
	v := trimmed
	return &v
}

func resolveUTCOrNow(value *time.Time, fallback time.Time) time.Time {
	if value == nil {
		return fallback
	}
	return value.UTC()
}

func resolveOptionalUTC(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	v := value.UTC()
	return &v
}

func isIdempotentCreateMatch(existing *Job, input *CreateJobInput, payload string) bool {
	if existing == nil || input == nil {
		return false
	}
	if existing.EventType != strings.TrimSpace(input.EventType) {
		return false
	}
	if existing.PayloadJSON != payload {
		return false
	}
	if !sameNullableInt64(existing.UserID, input.UserID) {
		return false
	}
	if !sameNullableInt64(existing.SubscriptionID, input.SubscriptionID) {
		return false
	}
	return true
}

func sameNullableInt64(left, right *int64) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	return *left == *right
}
