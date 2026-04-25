-- 持久化用户登录 Refresh Token，避免 Docker 镜像更新或 Redis 重启导致登录态丢失。
CREATE TABLE IF NOT EXISTS refresh_tokens (
    token_hash     VARCHAR(64) PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_version  BIGINT NOT NULL DEFAULT 0,
    family_id      VARCHAR(64) NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at     TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family_id ON refresh_tokens(family_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);
