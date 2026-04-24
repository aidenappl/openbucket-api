CREATE TABLE IF NOT EXISTS users (
    id                BIGINT       NOT NULL AUTO_INCREMENT PRIMARY KEY,
    email             VARCHAR(254) NOT NULL,
    name              VARCHAR(255),
    auth_type         VARCHAR(20)  NOT NULL DEFAULT 'local',
    password_hash     TEXT,
    sso_subject       VARCHAR(255),
    profile_image_url TEXT,
    role              VARCHAR(20)  NOT NULL DEFAULT 'viewer',
    active            TINYINT(1)   NOT NULL DEFAULT 1,
    updated_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    inserted_at       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_users_email_auth_type (email, auth_type),
    INDEX idx_users_active (active)
);
