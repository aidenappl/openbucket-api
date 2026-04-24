CREATE TABLE IF NOT EXISTS sessions (
    id            BIGINT       NOT NULL AUTO_INCREMENT PRIMARY KEY,
    forta_user_id BIGINT       NOT NULL,
    bucket_name   VARCHAR(255) NOT NULL,
    nickname      VARCHAR(255) NOT NULL DEFAULT '',
    region        VARCHAR(100) NOT NULL,
    endpoint      TEXT         NOT NULL,
    access_key    TEXT,
    secret_key    TEXT,
    inserted_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_sessions_forta_user_id (forta_user_id)
);
