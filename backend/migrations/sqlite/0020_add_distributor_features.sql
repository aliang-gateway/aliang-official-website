CREATE TABLE IF NOT EXISTS als_distributor_user_bindings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    distributor_user_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    source TEXT NOT NULL DEFAULT 'manual',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id),
    FOREIGN KEY(distributor_user_id) REFERENCES als_users(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_distributor_user_bindings_distributor ON als_distributor_user_bindings(distributor_user_id);
CREATE INDEX IF NOT EXISTS idx_distributor_user_bindings_user ON als_distributor_user_bindings(user_id);

CREATE TABLE IF NOT EXISTS als_distributor_package_assignments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    distributor_user_id INTEGER NOT NULL,
    target_user_id INTEGER NOT NULL,
    tier_code TEXT NOT NULL,
    fulfillment_job_id INTEGER,
    status TEXT NOT NULL DEFAULT 'created',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(distributor_user_id) REFERENCES als_users(id) ON DELETE CASCADE,
    FOREIGN KEY(target_user_id) REFERENCES als_users(id) ON DELETE CASCADE,
    FOREIGN KEY(fulfillment_job_id) REFERENCES als_fulfillment_jobs(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_distributor_package_assignments_distributor ON als_distributor_package_assignments(distributor_user_id);
CREATE INDEX IF NOT EXISTS idx_distributor_package_assignments_target ON als_distributor_package_assignments(target_user_id);

CREATE TABLE IF NOT EXISTS als_user_usage_daily (
    user_id INTEGER NOT NULL,
    usage_date TEXT NOT NULL,
    request_count INTEGER NOT NULL DEFAULT 0,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    actual_cost_micros INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(user_id, usage_date),
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_usage_daily_user_date ON als_user_usage_daily(user_id, usage_date);
