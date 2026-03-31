CREATE TABLE IF NOT EXISTS als_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS als_api_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    label TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS als_tiers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS als_service_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    unit TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS als_tier_default_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tier_id INTEGER NOT NULL,
    service_item_id INTEGER NOT NULL,
    included_units INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tier_id, service_item_id),
    FOREIGN KEY(tier_id) REFERENCES als_tiers(id) ON DELETE CASCADE,
    FOREIGN KEY(service_item_id) REFERENCES als_service_items(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS als_subscriptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    tier_id INTEGER NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE,
    FOREIGN KEY(tier_id) REFERENCES als_tiers(id)
);

CREATE TABLE IF NOT EXISTS als_subscription_overrides (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    subscription_id INTEGER NOT NULL,
    service_item_id INTEGER NOT NULL,
    included_units INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(subscription_id, service_item_id),
    FOREIGN KEY(subscription_id) REFERENCES als_subscriptions(id) ON DELETE CASCADE,
    FOREIGN KEY(service_item_id) REFERENCES als_service_items(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS als_unit_prices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    service_item_id INTEGER NOT NULL,
    tier_id INTEGER,
    price_per_unit_micros INTEGER NOT NULL,
    currency TEXT NOT NULL DEFAULT 'USD',
    effective_from TIMESTAMP NOT NULL,
    effective_to TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(service_item_id, tier_id, effective_from),
    FOREIGN KEY(service_item_id) REFERENCES als_service_items(id) ON DELETE CASCADE,
    FOREIGN KEY(tier_id) REFERENCES als_tiers(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS als_usage_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    api_key_id INTEGER,
    service_item_id INTEGER NOT NULL,
    quantity INTEGER NOT NULL,
    usage_timestamp TIMESTAMP NOT NULL,
    metadata_json TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE,
    FOREIGN KEY(api_key_id) REFERENCES als_api_keys(id) ON DELETE SET NULL,
    FOREIGN KEY(service_item_id) REFERENCES als_service_items(id)
);

CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON als_api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON als_subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_tier_id ON als_subscriptions(tier_id);
CREATE INDEX IF NOT EXISTS idx_usage_records_user_id ON als_usage_records(user_id);
CREATE INDEX IF NOT EXISTS idx_usage_records_service_item_id ON als_usage_records(service_item_id);
CREATE INDEX IF NOT EXISTS idx_usage_records_timestamp ON als_usage_records(usage_timestamp);
