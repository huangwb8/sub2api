-- 为订阅套餐补充升级族与升级等级元数据。
-- 幂等执行：可重复运行。

ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS upgrade_family TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS upgrade_rank INTEGER NOT NULL DEFAULT 0;
