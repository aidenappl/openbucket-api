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
	return query.UpsertSSOSession(db.DB, userID, encAccess, encRefresh)
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
		Tokens:        ssolib.TokenSet{AccessToken: access, RefreshToken: refresh},
		LastCheckedAt: row.LastCheckedAt,
	}, nil
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
// token whose `iat` is not after the stamp, so access and refresh tokens die
// together. Migration 008 adds the column.
// ─────────────────────────────────────────────────────────────────────────────
func (s *SessionStore) RevokeLocalTokens(_ context.Context, userID int64) error {
	return query.RevokeUserTokens(db.DB, userID)
}
