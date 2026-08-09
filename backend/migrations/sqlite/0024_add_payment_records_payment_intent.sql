-- Capture the Stripe PaymentIntent on each payment record so adverse events
-- (charge.refunded, chargebacks) can be attributed to a user/tier. Stripe does
-- NOT propagate Checkout Session metadata onto the Charge, so we resolve
-- attribution via payment_intent instead. Populated on checkout.session.completed.
ALTER TABLE als_payment_records ADD COLUMN payment_intent TEXT;

CREATE INDEX IF NOT EXISTS idx_payment_records_payment_intent
    ON als_payment_records (payment_intent);
