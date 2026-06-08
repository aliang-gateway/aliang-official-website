ALTER TABLE als_distributor_package_assignments
    ADD COLUMN IF NOT EXISTS price_micros BIGINT NOT NULL DEFAULT 0;

UPDATE als_distributor_package_assignments dpa
SET price_micros = COALESCE(t.price_micros, 0)
FROM als_tiers t
WHERE dpa.tier_code = t.code
    AND dpa.price_micros = 0;

CREATE INDEX IF NOT EXISTS idx_distributor_package_assignments_distributor_created
    ON als_distributor_package_assignments(distributor_user_id, created_at);

CREATE INDEX IF NOT EXISTS idx_distributor_package_assignments_tier_code
    ON als_distributor_package_assignments(tier_code);
