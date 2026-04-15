-- 新增标准余额计费的盈利率与账号实际成本字段。
-- 幂等执行：可重复运行。

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS actual_cost_cny NUMERIC(20, 8),
    ADD COLUMN IF NOT EXISTS actual_cost_usage_usd NUMERIC(20, 10),
    ADD COLUMN IF NOT EXISTS actual_cost_updated_at TIMESTAMPTZ;

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS extra_profit_rate_percent NUMERIC(10, 4);

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS estimated_cost_cny NUMERIC(20, 8);
