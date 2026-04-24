ALTER TABLE sessions CHANGE COLUMN forta_user_id user_id BIGINT NOT NULL;
ALTER TABLE sessions DROP INDEX idx_sessions_forta_user_id;
ALTER TABLE sessions ADD INDEX idx_sessions_user_id (user_id);
