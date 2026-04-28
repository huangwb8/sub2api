CREATE TABLE IF NOT EXISTS user_risk_profiles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    score DECIMAL(10,4) NOT NULL DEFAULT 5.0000,
    status VARCHAR(32) NOT NULL DEFAULT 'healthy',
    consecutive_bad_days INTEGER NOT NULL DEFAULT 0,
    last_evaluated_at TIMESTAMPTZ NULL,
    last_warned_at TIMESTAMPTZ NULL,
    grace_period_started_at TIMESTAMPTZ NULL,
    locked_at TIMESTAMPTZ NULL,
    lock_reason TEXT NOT NULL DEFAULT '',
    last_evaluation_summary TEXT NOT NULL DEFAULT '',
    exempted BOOLEAN NOT NULL DEFAULT FALSE,
    exempted_at TIMESTAMPTZ NULL,
    exempted_by BIGINT NULL,
    exemption_reason TEXT NOT NULL DEFAULT '',
    unlocked_at TIMESTAMPTZ NULL,
    unlocked_by BIGINT NULL,
    unlock_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_risk_profiles_status ON user_risk_profiles(status);
CREATE INDEX IF NOT EXISTS idx_user_risk_profiles_exempted ON user_risk_profiles(exempted);
CREATE INDEX IF NOT EXISTS idx_user_risk_profiles_locked_at ON user_risk_profiles(locked_at);
