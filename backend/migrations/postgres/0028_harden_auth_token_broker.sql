ALTER TABLE als_sub2api_auth_tokens ADD COLUMN rotation_state TEXT NOT NULL DEFAULT 'stable';
ALTER TABLE als_sub2api_auth_tokens ADD COLUMN rotation_started_at TIMESTAMPTZ;
ALTER TABLE als_sub2api_auth_tokens ADD COLUMN version BIGINT NOT NULL DEFAULT 0;
