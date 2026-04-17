-- 为用户订阅补充套餐快照与计费周期起点，支持升级补差价精算。
-- 幂等执行：可重复运行。

ALTER TABLE user_subscriptions
    ADD COLUMN IF NOT EXISTS current_plan_id BIGINT,
    ADD COLUMN IF NOT EXISTS current_plan_name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS current_plan_price_cny NUMERIC(20,2),
    ADD COLUMN IF NOT EXISTS current_plan_validity_days INTEGER,
    ADD COLUMN IF NOT EXISTS current_plan_validity_unit VARCHAR(10) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS billing_cycle_started_at TIMESTAMPTZ;
