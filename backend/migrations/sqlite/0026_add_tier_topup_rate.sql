-- Tier top-up rate: for balance (加量包) tiers, user pays any CNY amount,
-- and the sub2api balance is credited as paid_cny × rate (in USD).
-- rate is NULL/0 for fixed-amount balance tiers (legacy) and days tiers.

ALTER TABLE als_tiers ADD COLUMN rate NUMERIC(6,2);
ALTER TABLE als_tiers ADD COLUMN min_topup_micros BIGINT;
ALTER TABLE als_tiers ADD COLUMN max_topup_micros BIGINT;
