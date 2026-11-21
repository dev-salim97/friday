-- Manual MySQL migration: CodeBuddy support was removed from the platform.
-- Run once after deploying. Startup never drops columns or tables.

DROP TABLE IF EXISTS platform_codebuddy_keys;

ALTER TABLE user_configs DROP COLUMN codebuddy_models;
ALTER TABLE user_configs DROP COLUMN codebuddy_api_key;
ALTER TABLE user_configs DROP COLUMN codebuddy_internet_environment;

-- Definitions that still point at the removed executor fall back to Claude Code.
UPDATE agents SET payload = JSON_SET(payload, '$.executorId', 'claude')
WHERE JSON_UNQUOTE(JSON_EXTRACT(payload, '$.executorId')) = 'codebuddy';
