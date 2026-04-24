CREATE TABLE IF NOT EXISTS instances (
    id          BIGINT       NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    endpoint    TEXT         NOT NULL,
    admin_token TEXT         NOT NULL,
    active      TINYINT(1)   NOT NULL DEFAULT 1,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    inserted_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_instances_active (active)
);
