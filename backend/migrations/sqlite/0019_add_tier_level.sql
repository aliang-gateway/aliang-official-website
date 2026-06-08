ALTER TABLE als_tiers ADD COLUMN level TEXT NOT NULL DEFAULT 'admin';

UPDATE als_tiers
SET level = 'admin'
WHERE level IS NULL OR level = '';
