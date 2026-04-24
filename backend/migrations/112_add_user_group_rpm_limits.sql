-- Add RPM limits for user/group/user-group scopes.
-- NULL = inherit/not configured, 0 = unlimited, positive = requests per minute.
ALTER TABLE users ADD COLUMN IF NOT EXISTS rpm_limit INT DEFAULT NULL;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS rpm_limit INT DEFAULT NULL;
ALTER TABLE user_allowed_groups ADD COLUMN IF NOT EXISTS rpm_limit INT DEFAULT NULL;
