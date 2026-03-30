CREATE TABLE IF NOT EXISTS als_tier_group_bindings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tier_id INTEGER NOT NULL,
    group_code TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tier_id, group_code),
    FOREIGN KEY(tier_id) REFERENCES als_tiers(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_tier_group_bindings_tier_id ON als_tier_group_bindings(tier_id);
CREATE INDEX IF NOT EXISTS idx_tier_group_bindings_group_code ON als_tier_group_bindings(group_code);

CREATE TABLE IF NOT EXISTS als_fulfillment_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    subscription_id INTEGER,
    event_type TEXT NOT NULL,
    status TEXT NOT NULL,
    payload_json TEXT,
    error_message TEXT,
    available_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    finished_at TIMESTAMP,
    retry_count INTEGER NOT NULL DEFAULT 0,
    idempotency_key TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE SET NULL,
    FOREIGN KEY(subscription_id) REFERENCES als_subscriptions(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_fulfillment_jobs_idempotency_key_non_null ON als_fulfillment_jobs(idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_fulfillment_jobs_status_available_at ON als_fulfillment_jobs(status, available_at);
CREATE INDEX IF NOT EXISTS idx_fulfillment_jobs_subscription_id ON als_fulfillment_jobs(subscription_id);

CREATE TABLE IF NOT EXISTS als_fulfillment_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    fulfillment_job_id INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    payload_json TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(fulfillment_job_id) REFERENCES als_fulfillment_jobs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_fulfillment_events_job_id ON als_fulfillment_events(fulfillment_job_id);
