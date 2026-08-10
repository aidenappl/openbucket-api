package routers

import (
	"net/http"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/jwt"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
)

func HandleRefresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("ob-refresh-token")
	if err != nil || cookie.Value == "" {
		responder.SendError(w, http.StatusUnauthorized, "no refresh token")
		return
	}

	claims, err := jwt.ValidateRefreshTokenClaims(cookie.Value)
	if err != nil {
		responder.SendError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	user, err := query.GetUserByID(db.DB, claims.UserID)
	if err != nil || user == nil || !user.Active {
		responder.SendError(w, http.StatusUnauthorized, "user not found or inactive")
		return
	}

	// ─────────────────────────────────────────────────────────────────────────
	// ⚠️ WITHOUT THIS, REVOCATION UNDOES ITSELF IN ABOUT ONE SECOND.
	//
	// SessionStore.RevokeLocalTokens stamps users.tokens_revoked_at, and
	// middleware/auth.go rejects any token whose `iat` is not after the stamp.
	// That correctly kills the ACCESS token — and the client immediately calls
	// this endpoint, which used to check only the signature and user.Active. It
	// then minted a brand-new pair whose `iat` is necessarily AFTER the stamp,
	// so the user was fully signed back in.
	//
	// Observed in production 2026-08-10: back-channel logout reported
	// "ended 1 session(s)", two requests 401'd, /auth/refresh succeeded one
	// second later, and the session continued. Revocation — by grant withdrawal,
	// by the introspection checkpoint, or by push logout — was defeated by this
	// endpoint alone.
	//
	// Mirrors the comparison in middleware/auth.go; the two must not diverge.
	// ─────────────────────────────────────────────────────────────────────────
	if user.TokensRevokedAt != nil && claims.IssuedAt != nil &&
		!claims.IssuedAt.Time.After(*user.TokensRevokedAt) {
		responder.SendError(w, http.StatusUnauthorized, "session revoked")
		return
	}

	accessToken, accessExpiry, err := jwt.NewAccessToken(user.ID)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to generate access token")
		return
	}

	refreshToken, refreshExpiry, err := jwt.NewRefreshToken(user.ID)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}

	setTokenCookies(w, accessToken, refreshToken, accessExpiry, refreshExpiry)
	responder.New(w, user, "token refreshed")
}
