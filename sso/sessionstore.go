package sso

import (
	"context"
	"fmt"

	ssolib "github.com/aidenappl/go-forta/sso"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/tools"
)

// SessionStore implements ssolib.SessionStore over sso_sessions, with AES-256-GCM
// encryption at rest.
type SessionStore struct{}

// NewSessionStore returns a SessionStore over the package-level DB handle.
func NewSessionStore() *SessionStore { return &SessionStore{} }

// SaveSession encrypts and upserts the IdP tokens for a user.
func (s *SessionStore) SaveSession(_ context.Context, userID int64, sess ssolib.Session) error {
	encAccess, err := tools.Encrypt(sess.Tokens.AccessToken)
	if err != nil {
		return fmt.Errorf("sso: encrypt access token: %w", err)
	}

	encRefresh := ""
	if sess.Tokens.RefreshToken != "" {
		encRefresh, err = tools.Encrypt(sess.Tokens.RefreshToken)
		if err != nil {
			return fmt.Errorf("sso: encrypt refresh token: %w", err)
		}
	}

	// refresh_token is nullable as of migration 009. It was NOT NULL, which meant a
	// provider issuing no refresh token either stored an empty string —
	// indistinguishable from an encrypted empty value — or failed the insert and
	// silently left the session uncheckpointed.
	// Subject and SID are persisted even though nothing reads them until a logout
	// token arrives. Neither can be added later: `sid` lives only in the id_token
	// of the login that created this row. It stays empty while sso.issuer_url is
	// unset, because the OAuth2 adapter has no id_token at all.
	return query.UpsertSSOSession(db.DB, userID, ProviderSlug, sess.Subject, sess.SID, encAccess, encRefresh)
}

// LoadSession returns the decrypted session, or (nil, nil) when the user has none.
//
// ⚠️ (nil, nil) MUST NOT BECOME AN ERROR. OpenBucket has local accounts, and a
// local login has no row here. The checkpoint reads (nil, nil) as "not an SSO
// session, pass"; an error would deny every local login.
func (s *SessionStore) LoadSession(_ context.Context, userID int64) (*ssolib.Session, error) {
	row, err := query.GetSSOSession(db.DB, userID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}

	access, err := tools.Decrypt(row.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("sso: decrypt access token for user %d: %w", userID, err)
	}

	refresh := ""
	if row.RefreshToken != "" {
		refresh, err = tools.Decrypt(row.RefreshToken)
		if err != nil {
			return nil, fmt.Errorf("sso: decrypt refresh token for user %d: %w", userID, err)
		}
	}

	return &ssolib.Session{
		Provider:      ProviderSlug,
		Subject:       derefOr(row.Subject),
		SID:           derefOr(row.SID),
		Tokens:        ssolib.TokenSet{AccessToken: access, RefreshToken: refresh},
		LastCheckedAt: row.LastCheckedAt,
	}, nil
}

// derefOr flattens a nullable column. NULL and "" mean the same thing to every
// caller here: this session cannot be addressed that way.
func derefOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ─────────────────────────────────────────────────────────────────────────────
// BackchannelLogoutTarget — the receiving half of OIDC Back-Channel Logout 1.0.
//
// ⚠️ DELETING THE ROW IS NOT ENOUGH HERE, AND THIS IS WHERE OPENBUCKET DIFFERS
// FROM A SERVICE THAT VALIDATES EVERY REQUEST AGAINST ITS SESSION TABLE.
//
// OpenBucket issues its OWN JWTs. They outlive the sso_sessions row, so a
// back-channel logout that only deleted rows would end nothing the user notices —
// the identical bug RevokeLocalTokens exists to fix on the checkpoint path,
// reintroduced through a new one. go-forta's handler does not call
// RevokeLocalTokens (it has no way to know it is needed), so stamping
// tokens_revoked_at is this implementation's job.
//
// The user ids are therefore read BEFORE the delete: afterwards they are
// unrecoverable.
//
// ⚠️ Both methods return (0, nil) for "nothing matched", never an error. A
// duplicate delivery, an expired session and a pre-migration row all land here and
// all are normal; an error would make the provider retry a message it had already
// applied and then report this receiver as broken.
// ─────────────────────────────────────────────────────────────────────────────

// DeleteSessionsBySID ends the single session with this OIDC session id.
func (s *SessionStore) DeleteSessionsBySID(ctx context.Context, provider, sid string) (int, error) {
	ids, err := query.SSOSessionUserIDsBySID(db.DB, provider, sid)
	if err != nil {
		return 0, err
	}
	n, err := query.DeleteSSOSessionsBySID(db.DB, provider, sid)
	if err != nil {
		return 0, err
	}
	return n, s.revokeAll(ctx, ids)
}

// DeleteSessionsBySubject ends every session this subject holds with the provider
// — the correct scope for a subject-wide event such as a revoked grant.
func (s *SessionStore) DeleteSessionsBySubject(ctx context.Context, provider, subject string) (int, error) {
	ids, err := query.SSOSessionUserIDsBySubject(db.DB, provider, subject)
	if err != nil {
		return 0, err
	}
	n, err := query.DeleteSSOSessionsBySubject(db.DB, provider, subject)
	if err != nil {
		return 0, err
	}
	return n, s.revokeAll(ctx, ids)
}

// revokeAll stamps tokens_revoked_at for every affected user.
//
// A failure here is returned, not swallowed: the session row is already gone, so
// if this does not land the user keeps working local tokens and the revocation
// silently did nothing. Returning the error makes the receiver answer 500, which
// makes the provider RETRY — the one case where a retry is exactly right.
func (s *SessionStore) revokeAll(ctx context.Context, userIDs []int64) error {
	for _, id := range userIDs {
		if err := s.RevokeLocalTokens(ctx, id); err != nil {
			return fmt.Errorf("sso: revoke local tokens for user %d: %w", id, err)
		}
	}
	return nil
}

// TouchSession resets the checkpoint interval after a successful check.
func (s *SessionStore) TouchSession(_ context.Context, userID int64) error {
	return query.TouchSSOSession(db.DB, userID)
}

// DeleteSession removes the SSO session row.
func (s *SessionStore) DeleteSession(_ context.Context, userID int64) error {
	return query.DeleteSSOSession(db.DB, userID)
}

// RevokeLocalTokens satisfies ssolib.LocalTokenRevoker.
//
// ─────────────────────────────────────────────────────────────────────────────
// ⚠️ WITHOUT THIS, REVOCATION LASTED EXACTLY ONE REQUEST.
//
// The old checkpoint responded to an inactive grant by deleting the sso_sessions
// row and returning false. That 401'd the request in flight — and the NEXT request
// found no row, took the "not an SSO session, allow" branch, and let the user
// straight back in for the full life of their existing OpenBucket JWTs.
//
// Stamping tokens_revoked_at is what bites: middleware.validateToken rejects any
// token whose `iat` is not after the stamp. Migration 008 adds the column.
//
// ⚠️ THAT IS ONLY HALF THE STORY, AND THE OTHER HALF WAS MISSING UNTIL
// 2026-08-10. Killing the access token accomplishes nothing on its own, because
// the client immediately calls /auth/refresh — and that handler checked only the
// signature and user.Active, so it minted a fresh pair whose `iat` is
// necessarily AFTER the stamp. The revocation undid itself in about a second.
// HandleRefresh now performs the same comparison; the two must not diverge.
// ─────────────────────────────────────────────────────────────────────────────
func (s *SessionStore) RevokeLocalTokens(_ context.Context, userID int64) error {
	return query.RevokeUserTokens(db.DB, userID)
}
