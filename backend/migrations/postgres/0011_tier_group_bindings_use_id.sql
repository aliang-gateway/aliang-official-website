-- PostgreSQL: rename group_code to group_id and change type to BIGINT
ALTER TABLE als_tier_group_bindings RENAME COLUMN group_code TO group_id;
ALTER TABLE als_tier_group_bindings ALTER COLUMN group_id TYPE BIGINT USING group_id::bigint;
ALTER TABLE als_tier_group_bindings ALTER COLUMN group_id SET NOT NULL;

DROP INDEX IF EXISTS idx_tier_group_bindings_group_code;
CREATE INDEX idx_tier_group_bindings_group_id ON als_tier_group_bindings(group_id);
