-- Add short-lived proxy probe history for reliability analysis.
CREATE TABLE IF NOT EXISTS proxy_probe_logs (
    id BIGSERIAL PRIMARY KEY,
    proxy_id BIGINT NOT NULL,
    source VARCHAR(64) NOT NULL DEFAULT 'scheduled_probe',
    target VARCHAR(64) NOT NULL DEFAULT 'probe_chain',
    success BOOLEAN NOT NULL DEFAULT FALSE,
    latency_ms BIGINT,
    error_message VARCHAR(1024),
    ip_address VARCHAR(45),
    country_code VARCHAR(16),
    country VARCHAR(100),
    region VARCHAR(100),
    city VARCHAR(100),
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proxy_probe_logs_proxy_checked_at
    ON proxy_probe_logs (proxy_id, checked_at DESC);

CREATE INDEX IF NOT EXISTS idx_proxy_probe_logs_success_checked_at
    ON proxy_probe_logs (success, checked_at DESC);

CREATE INDEX IF NOT EXISTS idx_proxy_probe_logs_source_checked_at
    ON proxy_probe_logs (source, checked_at DESC);
