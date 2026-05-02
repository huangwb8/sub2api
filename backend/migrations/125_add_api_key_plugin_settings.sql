ALTER TABLE api_keys
ADD COLUMN IF NOT EXISTS plugin_settings JSONB NOT NULL DEFAULT '{}'::jsonb;
