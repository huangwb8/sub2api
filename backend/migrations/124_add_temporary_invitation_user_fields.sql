ALTER TABLE users
    ADD COLUMN IF NOT EXISTS temporary_invitation BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS temporary_invitation_deadline_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS temporary_invitation_disabled_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS temporary_invitation_delete_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_users_temporary_invitation
    ON users (temporary_invitation);

CREATE INDEX IF NOT EXISTS idx_users_temporary_invitation_delete_at
    ON users (temporary_invitation_delete_at);
