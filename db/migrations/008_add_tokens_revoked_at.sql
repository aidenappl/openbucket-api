-- 008_add_tokens_revoked_at.sql
--
-- ─────────────────────────────────────────────────────────────────────────────
-- FIXES REVOCATION THAT DID NOT STICK.
--
-- When the identity provider reported a grant inactive, checkpointSSOGrant deleted
-- the sso_sessions row and returned false — so THAT request 401'd. The next
-- request found no row, took the `sess == nil` branch, and was ALLOWED. A revoked
-- user was locked out for exactly one request and then kept working for the full
-- lifetime of their existing OpenBucket JWTs.
--
-- Deleting the session cannot lock anyone out on its own, because OpenBucket
-- issues its own JWTs and validates them locally with no reference to that row.
-- This column is the thing that bites: validateToken rejects any token issued
-- before the stamp, so revocation takes effect across every token the user holds,
-- immediately.
--
-- Nullable with no default: NULL means "nothing revoked", which is every existing
-- user. A DEFAULT of CURRENT_TIMESTAMP would revoke the entire estate on migration.
-- ─────────────────────────────────────────────────────────────────────────────

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS tokens_revoked_at DATETIME NULL DEFAULT NULL;
