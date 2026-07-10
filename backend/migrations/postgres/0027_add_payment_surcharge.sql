-- Payment surcharge (handling fee): track per-record fee + seed admin-configurable thresholds.
-- fee_minor records the fee portion (currency minor, e.g. CNY fen) so fulfillment can compute
-- the rechargeable base as (amount_minor - fee_minor), keeping the fee out of the credited balance.

ALTER TABLE als_payment_records ADD COLUMN fee_minor BIGINT NOT NULL DEFAULT 0;

INSERT INTO als_global_template_vars (var_key, var_value, description) VALUES
('payment_surcharge_enabled', 'true', 'Enable handling fee for small-checkout payments (true/false)'),
('payment_surcharge_amount', '3', 'Handling fee amount in CNY yuan, charged when payment < threshold'),
('payment_surcharge_threshold', '50', 'Threshold in CNY yuan; orders strictly below this incur the fee')
ON CONFLICT (var_key) DO NOTHING;
