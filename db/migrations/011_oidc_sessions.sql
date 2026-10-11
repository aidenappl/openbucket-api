-- 011_oidc_sessions.sql
--
-- SSO sessions and login states for go-oidc-session.
--
-- oidc_sessions: one row per signed-in browser, keyed by the SHA-256 of the
-- openbucket_session cookie, holding the provider's tokens encrypted with the
-- service key (tools.Encrypt). The auth middleware refreshes them against the
-- provider when due and deletes the row when the provider refuses, so a session
-- lives exactly as long as the provider session.
--
-- oidc_login_states: one row per login in flight (PKCE verifier, nonce, return
-- URL), consumed once by the callback.
--
-- Replaces sso_sessions (one row per user, never refreshed, read only by the
-- removed introspection checkpoint) and the sso_state:* settings rows.
-- sso_sessions is left in place, unused, rather than dropped here.
--
-- Shape is go-oidc-session's sqlstore tables.

CREATE TABLE IF NOT EXISTS oidc_sessions (
    id            CHAR(64)     NOT NULL,
    user_id       BIGINT       NOT NULL,
    provider      VARCHAR(64)  NOT NULL,
    subject       VARCHAR(255) NOT NULL,
    sid           VARCHAR(255) NULL,
    access_token  TEXT         NOT NULL,
    refresh_token TEXT         NULL,
    id_token      TEXT         NULL,
    token_expiry  DATETIME(6)  NULL,
    created_at    DATETIME(6)  NOT NULL,
    last_seen_at  DATETIME(6)  NOT NULL,
    refreshed_at  DATETIME(6)  NOT NULL,
    expires_at    DATETIME(6)  NOT NULL,
    PRIMARY KEY (id),
    KEY idx_user (user_id),
    KEY idx_subject (provider, subject),
    KEY idx_sid (provider, sid),
    KEY idx_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS oidc_login_states (
    state      VARCHAR(64) NOT NULL,
    data       TEXT        NOT NULL,
    expires_at DATETIME(6) NOT NULL,
    PRIMARY KEY (state),
    KEY idx_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
