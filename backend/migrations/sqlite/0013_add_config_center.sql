-- Config Center: software configs, tags, templates, global vars, user synced configs

CREATE TABLE IF NOT EXISTS als_software_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    software_code TEXT NOT NULL UNIQUE,
    software_name TEXT NOT NULL,
    group_id INTEGER NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_enabled INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_software_configs_code ON als_software_configs(software_code);

CREATE TABLE IF NOT EXISTS als_software_tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    software_config_id INTEGER NOT NULL,
    tag TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(software_config_id) REFERENCES als_software_configs(id) ON DELETE CASCADE,
    UNIQUE(software_config_id, tag)
);
CREATE INDEX IF NOT EXISTS idx_software_tags_tag ON als_software_tags(tag);

CREATE TABLE IF NOT EXISTS als_config_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    software_config_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    format TEXT NOT NULL DEFAULT 'json',
    content TEXT NOT NULL,
    is_default INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(software_config_id) REFERENCES als_software_configs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS als_global_template_vars (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    var_key TEXT NOT NULL UNIQUE,
    var_value TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS als_user_synced_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    uuid TEXT NOT NULL,
    software TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    file_path TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    in_use INTEGER NOT NULL DEFAULT 0,
    selected INTEGER NOT NULL DEFAULT 0,
    format TEXT NOT NULL DEFAULT 'json',
    content TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE,
    UNIQUE(user_id, uuid)
);
CREATE INDEX IF NOT EXISTS idx_user_synced_configs_user ON als_user_synced_configs(user_id);
CREATE INDEX IF NOT EXISTS idx_user_synced_configs_software ON als_user_synced_configs(user_id, software);
