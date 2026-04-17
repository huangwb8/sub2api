-- 为订阅升级订单补充来源订阅与差价冻结快照字段。
-- 幂等执行：可重复运行。

ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS source_subscription_id BIGINT,
    ADD COLUMN IF NOT EXISTS source_plan_id BIGINT,
    ADD COLUMN IF NOT EXISTS upgrade_credit_cny NUMERIC(20,2),
    ADD COLUMN IF NOT EXISTS upgrade_payable_cny NUMERIC(20,2),
    ADD COLUMN IF NOT EXISTS upgrade_remaining_ratio NUMERIC(10,4);
