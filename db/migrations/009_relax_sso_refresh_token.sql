-- 009_relax_sso_refresh_token.sql
--
-- sso_sessions.refresh_token was NOT NULL. A provider that issues no refresh token
-- is legitimate — nothing in OAuth2 requires one — and monitor-core and lattice-api
-- both allow it null. Under NOT NULL, such a login either stored an empty string
-- (indistinguishable from "encrypted empty value") or failed the insert outright,
-- silently leaving the session uncheckpointed.
--
-- The checkpoint already prefers the refresh token and falls back to the access
-- token, so a null here degrades revocation latency rather than breaking it.
--
-- MODIFY is idempotent: re-running it against an already-nullable column is a
-- no-op, which is what makes this safe under the at-least-once migration runner.

ALTER TABLE sso_sessions
    MODIFY COLUMN refresh_token TEXT NULL;
