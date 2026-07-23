CREATE TABLE IF NOT EXISTS api_tokens (
    id           BIGINT       NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id      BIGINT       NOT NULL,
    name         VARCHAR(255) NOT NULL,
    token_hash   CHAR(64)     NOT NULL,
    expires_at   DATETIME     NULL,
    last_used_at DATETIME     NULL,
    active       TINYINT(1)   NOT NULL DEFAULT 1,
    updated_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    inserted_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_api_tokens_token_hash (token_hash),
    INDEX idx_api_tokens_user_active (user_id, active)
);
