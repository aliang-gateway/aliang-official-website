-- Worker polls failed_retryable jobs whose available_at has passed. Index the
-- (status, available_at) predicate so the background sweep stays cheap as the
-- table grows.
CREATE INDEX IF NOT EXISTS idx_fulfillment_jobs_retry
    ON als_fulfillment_jobs (status, available_at);
