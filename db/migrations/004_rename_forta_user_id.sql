ALTER TABLE sessions CHANGE COLUMN IF EXISTS forta_user_id user_id BIGINT NOT NULL;
ALTER TABLE sessions DROP INDEX IF EXISTS idx_sessions_forta_user_id;
ALTER TABLE sessions ADD INDEX IF NOT EXISTS idx_sessions_user_id (user_id);
