-- Config Center: software configs, tags, templates, global vars, user synced configs

CREATE TABLE IF NOT EXISTS als_software_configs (
    id BIGSERIAL PRIMARY KEY,
    software_code TEXT NOT NULL UNIQUE,
    software_name TEXT NOT NULL,
    group_id BIGINT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_software_configs_code ON als_software_configs(software_code);

CREATE TABLE IF NOT EXISTS als_software_tags (
    id BIGSERIAL PRIMARY KEY,
    software_config_id BIGINT NOT NULL,
    tag TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(software_config_id) REFERENCES als_software_configs(id) ON DELETE CASCADE,
    UNIQUE(software_config_id, tag)
);
CREATE INDEX IF NOT EXISTS idx_software_tags_tag ON als_software_tags(tag);

CREATE TABLE IF NOT EXISTS als_config_templates (
    id BIGSERIAL PRIMARY KEY,
    software_config_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    format TEXT NOT NULL DEFAULT 'json',
    content TEXT NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(software_config_id) REFERENCES als_software_configs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS als_global_template_vars (
    id BIGSERIAL PRIMARY KEY,
    var_key TEXT NOT NULL UNIQUE,
    var_value TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS als_user_synced_configs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    uuid TEXT NOT NULL,
    software TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    file_path TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    in_use BOOLEAN NOT NULL DEFAULT FALSE,
    selected BOOLEAN NOT NULL DEFAULT FALSE,
    format TEXT NOT NULL DEFAULT 'json',
    content TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE,
    UNIQUE(user_id, uuid)
);
CREATE INDEX IF NOT EXISTS idx_user_synced_configs_user ON als_user_synced_configs(user_id);
CREATE INDEX IF NOT EXISTS idx_user_synced_configs_software ON als_user_synced_configs(user_id, software);
