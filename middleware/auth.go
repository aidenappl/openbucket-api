package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	oidcsession "github.com/aidenappl/go-oidc-session"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/jwt"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/sso"
	"github.com/aidenappl/openbucket-api/structs"
	"github.com/aidenappl/openbucket-api/timing"
	"github.com/aidenappl/openbucket-api/tools"
)

// ssoCheckpointTTL controls how often the auth middleware re-validates an
// SSO user's grant against the IDP. Shorter = faster revocation propagation,
// more network calls. 5 min is the practical floor for an admin-initiated
// revoke since the IDP's access token TTL is 10 min.

type contextKey string

const (
	UserContextKey    contextKey = "user"
	obAccessToken                = "ob-access-token"
	SessionContextKey contextKey = "session"
)

// GetUserFromContext returns the authenticated user injected by AuthMiddleware.
func GetUserFromContext(ctx context.Context) (*structs.User, bool) {
	user, ok := ctx.Value(UserContextKey).(*structs.User)
	return user, ok
}

// GetUserID extracts the authenticated user's ID from context.
func GetUserID(ctx context.Context) (int, bool) {
	user, ok := GetUserFromContext(ctx)
	if !ok || user == nil {
		return 0, false
	}
	return user.ID, true
}

// AuthMiddleware checks authentication from either:
// 1. JWT via Authorization: Bearer header
// 2. JWT from ob-access-token cookie
// On success, injects *structs.User into the request context.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Carries the inbound ids to the SSO checkpoint only; the context handed
		// to next is unchanged.
		checkCtx := correlatedContext(r)

		// The auth span ends where the handler starts (or the request fails).
		endAuth := timing.Track(r.Context(), "auth")
		proceed := func(r *http.Request) {
			endAuth()
			next.ServeHTTP(w, r)
		}

		// Try Bearer token from Authorization header
		if token := extractBearerToken(r); token != "" {
			if user := validateToken(checkCtx, token); user != nil {
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				proceed(r.WithContext(ctx))
				return
			}
		}

		// Try JWT from cookie
		// SSO session. Authenticate refreshes the provider tokens when due; a
		// provider outage is 503, never 401 — a 401 would send the browser to
		// re-authenticate against the provider that is down.
		if sso.SessionRequest(r) {
			sess, err := sso.Manager().Authenticate(w, r.WithContext(checkCtx))
			if errors.Is(err, oidcsession.ErrProviderUnavailable) {
				responder.SendError(w, http.StatusServiceUnavailable, "identity provider unavailable, retry shortly")
				return
			}
			if err == nil {
				if user := sessionUser(sess); user != nil {
					ctx := context.WithValue(oidcsession.WithSession(r.Context(), sess), UserContextKey, user)
					proceed(r.WithContext(ctx))
					return
				}
			} else if !errors.Is(err, oidcsession.ErrNoSession) && !errors.Is(err, oidcsession.ErrSessionEnded) {
				logfCtx(checkCtx, "auth: sso session lookup failed: %v", err)
			}
		}

		if cookie, err := r.Cookie(obAccessToken); err == nil && cookie.Value != "" {
			if user := validateToken(checkCtx, cookie.Value); user != nil {
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				proceed(r.WithContext(ctx))
				return
			}
		}

		responder.SendError(w, http.StatusUnauthorized, "authentication required")
	})
}

// Protected wraps a single HandlerFunc with authentication.
func Protected(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		AuthMiddleware(http.HandlerFunc(next)).ServeHTTP(w, r)
	}
}

// RejectPending blocks users with role "pending" from accessing protected routes.
// Allows /auth/self so pending users can check their status.
func RejectPending(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r.Context())
		if ok && user != nil && user.Role == "pending" {
			responder.SendErrorWithParams(w, "your account is pending admin approval", http.StatusForbidden, intPtr(4004), nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin wraps a handler to require admin role.
func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r.Context())
		if !ok || user == nil {
			responder.SendError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if user.Role != "admin" {
			responder.SendError(w, http.StatusForbidden, "admin access required")
			return
		}
		next(w, r)
	}
}

// RequireEditor wraps a handler to require admin or editor role.
func RequireEditor(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r.Context())
		if !ok || user == nil {
			responder.SendError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if user.Role != "admin" && user.Role != "editor" {
			responder.SendError(w, http.StatusForbidden, "editor access required")
			return
		}
		next(w, r)
	}
}

// validateApiToken resolves an opaque API token (obk_…) to its owning user,
// rejecting tokens that are revoked, expired, or whose owner is inactive.
func validateApiToken(tokenStr string) *structs.User {
	apiToken, err := query.GetApiTokenByHash(db.DB, tools.HashApiToken(tokenStr))
	if err != nil || apiToken == nil {
		return nil
	}

	user, err := query.GetUserByID(db.DB, apiToken.UserID)
	if err != nil || user == nil || !user.Active {
		return nil
	}

	// Best-effort: a failed timestamp update must not fail the request.
	_ = query.TouchApiToken(db.DB, apiToken.ID)

	return user
}

// ctx is the request context, built by correlatedContext, so the SSO
// checkpoint's introspection carries the inbound request/trace ids.
func validateToken(ctx context.Context, tokenStr string) *structs.User {
	// Opaque API tokens are resolved against the database rather than parsed
	// as JWTs, so they are handled before JWT validation.
	if tools.IsApiToken(tokenStr) {
		return validateApiToken(tokenStr)
	}

	claims, err := jwt.ValidateAccessTokenClaims(tokenStr)
	if err != nil {
		return nil
	}
	userID := claims.UserID

	user, err := query.GetUserByID(db.DB, userID)
	if err != nil || user == nil || !user.Active {
		return nil
	}

	// ── Revocation cut-off ───────────────────────────────────────────────────
	//
	// ⚠️ THIS IS WHAT MAKES SSO REVOCATION STICK, and without it the checkpoint
	// below is nearly useless. Deleting the session row (oidc_sessions) 401s the request in
	// flight and nothing more: the next request finds no row, takes the "not an
	// SSO session, allow" branch, and the user is back in for the full life of
	// their tokens. Revocation was effective for exactly one request.
	//
	// Comparing `iat` against the stamp invalidates access and refresh tokens
	// together, with no per-token bookkeeping. It costs nothing — the user row was
	// already loaded above.
	//
	// NOT-BEFORE, not equality: a token issued in the same second as the stamp is
	// rejected. MySQL DATETIME has second granularity, so allowing equality would
	// leave a one-second window in which a token minted during the revocation
	// survives it.
	if user.TokensRevokedAt != nil && claims.IssuedAt != nil &&
		!claims.IssuedAt.Time.After(*user.TokensRevokedAt) {
		return nil
	}

	// SSO users authenticate with the openbucket_session cookie (sso.Manager),
	// whose lifetime is the identity provider's. An OpenBucket JWT for one is
	// from before that change; refusing it sends the browser through a silent
	// re-authentication that replaces it with a real session.
	if user.AuthType == "sso" {
		return nil
	}

	return user
}

// sessionUser loads the active user an SSO session belongs to.
func sessionUser(s *oidcsession.Session) *structs.User {
	user, err := query.GetUserByID(db.DB, int(s.UserID))
	if err != nil || user == nil || !user.Active {
		return nil
	}
	return user
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

func intPtr(v int) *int {
	return &v
}
