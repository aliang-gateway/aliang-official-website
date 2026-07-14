ALTER TABLE als_sub2api_auth_tokens ADD COLUMN rotation_state TEXT NOT NULL DEFAULT 'stable';
ALTER TABLE als_sub2api_auth_tokens ADD COLUMN rotation_started_at TIMESTAMP;
ALTER TABLE als_sub2api_auth_tokens ADD COLUMN version INTEGER NOT NULL DEFAULT 0;
