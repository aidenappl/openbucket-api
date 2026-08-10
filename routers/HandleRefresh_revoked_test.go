package routers

import (
	"testing"
	"time"

	"github.com/aidenappl/openbucket-api/jwt"
)

// TestRefreshTokenClaimsExposeIssuedAt guards the input the revocation check
// needs.
//
// ─────────────────────────────────────────────────────────────────────────────
// ⚠️ WITHOUT `iat` ON THE REFRESH PATH, REVOCATION UNDOES ITSELF.
//
// RevokeLocalTokens stamps users.tokens_revoked_at and middleware/auth.go
// rejects any token whose iat is not after it. That kills the ACCESS token — and
// the client immediately calls /auth/refresh. While that handler took only a
// user id (ValidateRefreshToken), it had nothing to compare, so it minted a
// fresh pair whose iat is necessarily AFTER the stamp and the user was signed
// straight back in.
//
// Observed in production 2026-08-10: back-channel logout logged
// "ended 1 session(s)", two requests 401'd, /auth/refresh succeeded one second
// later, and the session carried on. Every revocation path — grant withdrawal,
// the introspection checkpoint, and push logout — was defeated by that one
// endpoint.
//
// This pins that the claims-returning variant exists and carries IssuedAt, which
// is what HandleRefresh compares. Removing it, or reverting the handler to the
// id-only call, reopens the hole.
// ─────────────────────────────────────────────────────────────────────────────
func TestRefreshTokenClaimsExposeIssuedAt(t *testing.T) {
	token, _, err := jwt.NewRefreshToken(1)
	if err != nil {
		t.Fatalf("failed to mint a refresh token: %v", err)
	}

	claims, err := jwt.ValidateRefreshTokenClaims(token)
	if err != nil {
		t.Fatalf("ValidateRefreshTokenClaims rejected a freshly minted token: %v", err)
	}

	if claims.IssuedAt == nil {
		t.Fatal("refresh token claims carry no IssuedAt.\n\n" +
			"HandleRefresh compares it against users.tokens_revoked_at. With no iat " +
			"there is nothing to compare, so a revoked user refreshes straight back " +
			"in and every revocation path is defeated by the refresh endpoint.")
	}

	if claims.UserID != 1 {
		t.Fatalf("UserID = %d, want 1", claims.UserID)
	}

	// A token minted now must be usable: the comparison is `!iat.After(stamp)`,
	// so an iat in the future relative to a past revocation has to pass.
	past := time.Now().Add(-time.Hour)
	if !claims.IssuedAt.Time.After(past) {
		t.Fatal("a freshly minted token is not After an hour-old revocation stamp; " +
			"the comparison would lock out every valid session")
	}
}
