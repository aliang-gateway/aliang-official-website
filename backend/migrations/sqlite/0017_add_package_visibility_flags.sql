ALTER TABLE als_tiers ADD COLUMN is_visible INTEGER NOT NULL DEFAULT 1;
ALTER TABLE als_tiers ADD COLUMN is_published INTEGER NOT NULL DEFAULT 1;

UPDATE als_tiers
SET is_visible = is_enabled,
    is_published = is_enabled;
