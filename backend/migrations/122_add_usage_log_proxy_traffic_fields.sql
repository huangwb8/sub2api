ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS proxy_id BIGINT,
    ADD COLUMN IF NOT EXISTS used_residential_proxy BOOLEAN,
    ADD COLUMN IF NOT EXISTS proxy_traffic_input_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS proxy_traffic_output_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS proxy_traffic_overhead_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS proxy_traffic_estimate_source VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_usage_logs_proxy_id_created_at
    ON usage_logs (proxy_id, created_at)
    WHERE proxy_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_usage_logs_used_residential_proxy_created_at
    ON usage_logs (used_residential_proxy, created_at)
    WHERE used_residential_proxy IS NOT NULL;
