ALTER TABLE als_distributor_package_assignments ADD COLUMN price_micros INTEGER NOT NULL DEFAULT 0;

UPDATE als_distributor_package_assignments
SET price_micros = COALESCE((
    SELECT price_micros
    FROM als_tiers
    WHERE als_tiers.code = als_distributor_package_assignments.tier_code
), 0)
WHERE price_micros = 0;

CREATE INDEX IF NOT EXISTS idx_distributor_package_assignments_distributor_created
    ON als_distributor_package_assignments(distributor_user_id, created_at);

CREATE INDEX IF NOT EXISTS idx_distributor_package_assignments_tier_code
    ON als_distributor_package_assignments(tier_code);
