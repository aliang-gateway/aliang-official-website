ALTER TABLE als_users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT true;

CREATE TABLE IF NOT EXISTS als_email_verification_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    code TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_user_id ON als_email_verification_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_code ON als_email_verification_tokens(code);

CREATE TABLE IF NOT EXISTS als_email_outbox (
    id BIGSERIAL PRIMARY KEY,
    to_email TEXT NOT NULL,
    subject TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS als_user_wallets (
    user_id BIGINT PRIMARY KEY,
    balance_micros BIGINT NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS als_recharge_cards (
    id BIGSERIAL PRIMARY KEY,
    card_code TEXT NOT NULL UNIQUE,
    amount_micros BIGINT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'CNY',
    expires_at TIMESTAMPTZ,
    redeemed_by_user_id BIGINT,
    redeemed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(redeemed_by_user_id) REFERENCES als_users(id)
);

CREATE INDEX IF NOT EXISTS idx_recharge_cards_redeemed_by_user ON als_recharge_cards(redeemed_by_user_id);

CREATE TABLE IF NOT EXISTS als_wallet_transactions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    tx_type TEXT NOT NULL,
    amount_micros BIGINT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'CNY',
    balance_after_micros BIGINT NOT NULL,
    reference_type TEXT,
    reference_id BIGINT,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_wallet_transactions_user_id ON als_wallet_transactions(user_id);

CREATE TABLE IF NOT EXISTS als_user_profiles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    profile_name TEXT NOT NULL,
    profile_type TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT false,
    content_format TEXT NOT NULL,
    content_text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, profile_name),
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_profiles_user_id ON als_user_profiles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_profiles_user_type ON als_user_profiles(user_id, profile_type);
