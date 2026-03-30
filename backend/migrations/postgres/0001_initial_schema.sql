CREATE TABLE IF NOT EXISTS als_users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS als_api_keys (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    label TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMPTZ,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS als_tiers (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS als_service_items (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    unit TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS als_tier_default_items (
    id BIGSERIAL PRIMARY KEY,
    tier_id BIGINT NOT NULL,
    service_item_id BIGINT NOT NULL,
    included_units BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tier_id, service_item_id),
    FOREIGN KEY(tier_id) REFERENCES als_tiers(id) ON DELETE CASCADE,
    FOREIGN KEY(service_item_id) REFERENCES als_service_items(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS als_subscriptions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    tier_id BIGINT NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE,
    FOREIGN KEY(tier_id) REFERENCES als_tiers(id)
);

CREATE TABLE IF NOT EXISTS als_subscription_overrides (
    id BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT NOT NULL,
    service_item_id BIGINT NOT NULL,
    included_units BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(subscription_id, service_item_id),
    FOREIGN KEY(subscription_id) REFERENCES als_subscriptions(id) ON DELETE CASCADE,
    FOREIGN KEY(service_item_id) REFERENCES als_service_items(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS als_unit_prices (
    id BIGSERIAL PRIMARY KEY,
    service_item_id BIGINT NOT NULL,
    tier_id BIGINT,
    price_per_unit_micros BIGINT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'USD',
    effective_from TIMESTAMPTZ NOT NULL,
    effective_to TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(service_item_id, tier_id, effective_from),
    FOREIGN KEY(service_item_id) REFERENCES als_service_items(id) ON DELETE CASCADE,
    FOREIGN KEY(tier_id) REFERENCES als_tiers(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS als_usage_records (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT,
    service_item_id BIGINT NOT NULL,
    quantity BIGINT NOT NULL,
    usage_timestamp TIMESTAMPTZ NOT NULL,
    metadata_json TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
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
