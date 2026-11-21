-- Manual MySQL migration: remove CodeBuddy support. Run once by an operator;
-- the platform no longer reads or writes any of these columns/rows.

-- 1. Personal CodeBuddy credentials and model catalog (023/035).
ALTER TABLE user_configs
  DROP COLUMN codebuddy_api_key,
  DROP COLUMN codebuddy_internet_environment,
  DROP COLUMN codebuddy_models;

-- 2. Platform-shared CodeBuddy key (024).
DROP TABLE IF EXISTS platform_codebuddy_keys;

-- 3. The built-in CodeBuddy executor is gone; roles that pointed at it move to
--    the Claude Code executor ("claude").
DELETE FROM executors WHERE id = 'codebuddy' OR kind = 'codebuddy';

UPDATE agents
SET payload = JSON_SET(payload, '$.executorId', 'claude')
WHERE JSON_UNQUOTE(JSON_EXTRACT(payload, '$.executorId')) = 'codebuddy';

-- Workflow / template payloads embed their agents; rewrite both JSON spacings.
UPDATE workflows
SET payload = CAST(REPLACE(REPLACE(CAST(payload AS CHAR CHARACTER SET utf8mb4),
  '"executorId": "codebuddy"', '"executorId": "claude"'),
  '"executorId":"codebuddy"', '"executorId":"claude"') AS JSON)
WHERE CAST(payload AS CHAR CHARACTER SET utf8mb4) LIKE '%"executorId":%codebuddy"%';

UPDATE generic_templates
SET payload = CAST(REPLACE(REPLACE(CAST(payload AS CHAR CHARACTER SET utf8mb4),
  '"executorId": "codebuddy"', '"executorId": "claude"'),
  '"executorId":"codebuddy"', '"executorId":"claude"') AS JSON)
WHERE CAST(payload AS CHAR CHARACTER SET utf8mb4) LIKE '%"executorId":%codebuddy"%';

-- Runner runtime reports and historical run data are left untouched: stale
-- 'codebuddy' runtime entries disappear when runners re-register.
