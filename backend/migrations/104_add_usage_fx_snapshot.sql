-- 为 usage_logs 与 usage_billing_dedup 系列表补充人民币扣费与汇率快照字段。
-- 幂等执行：可重复运行。

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS charged_amount_cny NUMERIC(20, 8),
    ADD COLUMN IF NOT EXISTS fx_rate_usd_cny NUMERIC(20, 10),
    ADD COLUMN IF NOT EXISTS fx_rate_source TEXT,
    ADD COLUMN IF NOT EXISTS fx_fetched_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS fx_safety_margin NUMERIC(12, 6);

ALTER TABLE usage_billing_dedup
    ADD COLUMN IF NOT EXISTS balance_cost_cny NUMERIC(20, 8),
    ADD COLUMN IF NOT EXISTS fx_rate_usd_cny NUMERIC(20, 10),
    ADD COLUMN IF NOT EXISTS fx_rate_source TEXT,
    ADD COLUMN IF NOT EXISTS fx_fetched_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS fx_safety_margin NUMERIC(12, 6);

ALTER TABLE usage_billing_dedup_archive
    ADD COLUMN IF NOT EXISTS balance_cost_cny NUMERIC(20, 8),
    ADD COLUMN IF NOT EXISTS fx_rate_usd_cny NUMERIC(20, 10),
    ADD COLUMN IF NOT EXISTS fx_rate_source TEXT,
    ADD COLUMN IF NOT EXISTS fx_fetched_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS fx_safety_margin NUMERIC(12, 6);
