CREATE TABLE IF NOT EXISTS als_distributor_user_usage_snapshots (
    user_id BIGINT NOT NULL,
    upstream_user_id BIGINT NOT NULL DEFAULT 0,
    range_key TEXT NOT NULL DEFAULT 'all',
    request_count BIGINT NOT NULL DEFAULT 0,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    actual_cost_micros BIGINT NOT NULL DEFAULT 0,
    active_days BIGINT NOT NULL DEFAULT 0,
    last_active_date TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'sub2api',
    synced_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(user_id, range_key),
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_distributor_user_usage_snapshots_upstream
    ON als_distributor_user_usage_snapshots(upstream_user_id);

CREATE INDEX IF NOT EXISTS idx_distributor_user_usage_snapshots_synced
    ON als_distributor_user_usage_snapshots(synced_at);
