CREATE TABLE IF NOT EXISTS user_risk_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type VARCHAR(64) NOT NULL,
    severity VARCHAR(32) NOT NULL DEFAULT 'info',
    score_delta DECIMAL(10,4) NOT NULL DEFAULT 0,
    score_after DECIMAL(10,4) NOT NULL DEFAULT 0,
    summary TEXT NOT NULL DEFAULT '',
    metadata JSONB NULL DEFAULT '{}'::jsonb,
    window_start TIMESTAMPTZ NULL,
    window_end TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_risk_events_user_created_at ON user_risk_events(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_risk_events_type_created_at ON user_risk_events(event_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_risk_events_severity_created_at ON user_risk_events(severity, created_at DESC);
