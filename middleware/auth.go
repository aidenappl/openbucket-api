package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	ssolib "github.com/aidenappl/go-forta/sso"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/jwt"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/sso"
	"github.com/aidenappl/openbucket-api/structs"
	"github.com/aidenappl/openbucket-api/tools"
)

// ssoCheckpointTTL controls how often the auth middleware re-validates an
// SSO user's grant against the IDP. Shorter = faster revocation propagation,
// more network calls. 5 min is the practical floor for an admin-initiated
// revoke since the IDP's access token TTL is 10 min.
const ssoCheckpointTTL = 5 * time.Minute

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
		// Try Bearer token from Authorization header
		if token := extractBearerToken(r); token != "" {
			if user := validateToken(token); user != nil {
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// Try JWT from cookie
		if cookie, err := r.Cookie(obAccessToken); err == nil && cookie.Value != "" {
			if user := validateToken(cookie.Value); user != nil {
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))
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

func validateToken(tokenStr string) *structs.User {
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
	// below is nearly useless. Deleting the sso_sessions row 401s the request in
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

	if user.AuthType == "sso" && !checkpointSSOGrant(int64(userID)) {
		return nil
	}

	return user
}

// ssoCheckpointer is the shared checkpoint implementation, built once.
//
// The policy — 5-minute interval, 30-minute bounded grace measured from the last
// real answer, immediate action on a definitive active:false, and a hard
// distinction between "the IdP said no" and "the IdP did not answer" — lives in
// go-forta/sso so all three services share one copy of it.
var ssoCheckpointer = &ssolib.Checkpointer{
	Sessions: sso.NewSessionStore(),
	Providers: func(_ context.Context, slug string) (*ssolib.Provider, error) {
		if slug != sso.ProviderSlug {
			return nil, fmt.Errorf("auth: unknown sso provider %q", slug)
		}
		// Re-resolved every check, so a rotated secret or a newly-set introspect_url
		// takes effect at the next checkpoint rather than the next restart.
		return sso.LoadConfig().Provider(), nil
	},
	Interval: ssoCheckpointTTL,
	Logf:     log.Printf,
}

// checkpointSSOGrant re-validates the user's grant against the IdP on a TTL.
//
// ⚠️ WHAT CHANGED, AND WHY IT MATTERS MORE THAN IT LOOKS.
//
// The previous version failed open on ANY error with no bound — a permanently
// unreachable IdP meant permanently trusted sessions. It now fails open only inside
// a 30-minute window measured from the last positive confirmation, then denies.
//
// And on a definitive active:false it now stamps users.tokens_revoked_at through
// the library's LocalTokenRevoker, not just deletes the session row. Deleting the
// row alone made revocation last exactly ONE request: the next request found no row
// and took the "not an SSO session, allow" branch. See
// sso.SessionStore.RevokeLocalTokens.
//
// ⚠️ CheckpointUnavailable should be HTTP 503, not 401, and this bool signature
// cannot express it. A 401 sends clients to re-authenticate against the IdP that is
// already unreachable. Denying is still correct of the two options available, since
// allowing restores the unbounded fail-open. Widening the hook is the fix.
func checkpointSSOGrant(userID int64) bool {
	switch ssoCheckpointer.Check(context.Background(), userID) {
	case ssolib.CheckpointRevoked:
		log.Printf("checkpointSSOGrant: upstream grant revoked for user %d, session terminated", userID)
		return false
	case ssolib.CheckpointUnavailable:
		log.Printf("checkpointSSOGrant: unverifiable past grace window for user %d, denying (should be 503)", userID)
		return false
	default:
		return true
	}
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
