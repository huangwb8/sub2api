-- 为分组增加闲时动态计费配置（按北京时间秒级时间窗生效）
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS idle_rate_multiplier DECIMAL(10,4),
    ADD COLUMN IF NOT EXISTS idle_extra_profit_rate_percent DECIMAL(10,4),
    ADD COLUMN IF NOT EXISTS idle_start_seconds INTEGER,
    ADD COLUMN IF NOT EXISTS idle_end_seconds INTEGER;
