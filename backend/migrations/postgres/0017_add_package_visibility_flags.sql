ALTER TABLE als_tiers ADD COLUMN is_visible BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE als_tiers ADD COLUMN is_published BOOLEAN NOT NULL DEFAULT TRUE;

UPDATE als_tiers
SET is_visible = is_enabled,
    is_published = is_enabled;
