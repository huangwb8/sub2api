-- Add user avatar preferences and upload/external URL storage.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS avatar_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS avatar_type VARCHAR(32) NOT NULL DEFAULT 'generated',
    ADD COLUMN IF NOT EXISTS avatar_style VARCHAR(32) NOT NULL DEFAULT 'classic_letter';

UPDATE users
SET avatar_type = 'generated'
WHERE avatar_type IS NULL OR avatar_type = '';

UPDATE users
SET avatar_style = 'classic_letter'
WHERE avatar_style IS NULL OR avatar_style = '';

COMMENT ON COLUMN users.avatar_url IS '用户头像 URL；可为外链或 /uploads/avatars 下的本地上传路径';
COMMENT ON COLUMN users.avatar_type IS '头像来源：generated/external/uploaded';
COMMENT ON COLUMN users.avatar_style IS '生成头像风格';
