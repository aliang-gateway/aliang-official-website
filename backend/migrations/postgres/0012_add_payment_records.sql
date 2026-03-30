CREATE TABLE IF NOT EXISTS als_payment_records (
    id BIGSERIAL PRIMARY KEY,
    provider TEXT NOT NULL,
    checkout_session_id TEXT UNIQUE,
    payment_event_id TEXT UNIQUE,
    user_id BIGINT NOT NULL,
    tier_code TEXT NOT NULL,
    package_name TEXT NOT NULL DEFAULT '',
    customer_email TEXT NOT NULL DEFAULT '',
    amount_minor BIGINT NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    fulfillment_job_id BIGINT,
    payload_json TEXT NOT NULL DEFAULT '',
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE,
    FOREIGN KEY(fulfillment_job_id) REFERENCES als_fulfillment_jobs(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_payment_records_user_id ON als_payment_records(user_id);
CREATE INDEX IF NOT EXISTS idx_payment_records_status ON als_payment_records(status);
CREATE INDEX IF NOT EXISTS idx_payment_records_fulfillment_job_id ON als_payment_records(fulfillment_job_id);
