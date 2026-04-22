ALTER TABLE payment_orders
ADD COLUMN IF NOT EXISTS provider_key VARCHAR(30),
ADD COLUMN IF NOT EXISTS provider_snapshot JSONB;
