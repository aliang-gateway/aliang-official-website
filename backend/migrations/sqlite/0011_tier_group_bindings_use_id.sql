-- SQLite: rename group_code to group_id (BIGINT) via recreate
CREATE TABLE tier_group_bindings_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tier_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tier_id, group_id),
    FOREIGN KEY(tier_id) REFERENCES als_tiers(id) ON DELETE CASCADE
);

INSERT INTO tier_group_bindings_new (id, tier_id, group_id, created_at, updated_at)
    SELECT id, tier_id, 0, created_at, updated_at FROM als_tier_group_bindings;

DROP TABLE als_tier_group_bindings;

ALTER TABLE tier_group_bindings_new RENAME TO als_tier_group_bindings;

CREATE INDEX idx_tier_group_bindings_tier_id ON als_tier_group_bindings(tier_id);
CREATE INDEX idx_tier_group_bindings_group_id ON als_tier_group_bindings(group_id);
