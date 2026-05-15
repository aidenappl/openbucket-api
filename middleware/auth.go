package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

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

func validateToken(tokenStr string) *structs.User {
	userID, err := jwt.ValidateAccessToken(tokenStr)
	if err != nil {
		return nil
	}

	user, err := query.GetUserByID(db.DB, userID)
	if err != nil || user == nil || !user.Active {
		return nil
	}

	if user.AuthType == "sso" && !checkpointSSOGrant(int64(userID)) {
		return nil
	}

	return user
}

// checkpointSSOGrant re-validates the user's grant against the IDP on a TTL.
// Returns true if the grant is still active (or the check is skipped because
// it ran recently). Returns false if the IDP reports active=false — in which
// case the sso_sessions row is deleted and the caller MUST 401 the request,
// killing the local session.
//
// Network errors fail-open (return true) — a transient IDP outage shouldn't
// log users out, but it does mean revocation latency gets a small extra
// budget during incidents.
func checkpointSSOGrant(userID int64) bool {
	sess, err := query.GetSSOSession(db.DB, userID)
	if err != nil {
		log.Printf("checkpointSSOGrant: db lookup: %v (allowing request)", err)
		return true
	}
	if sess == nil {
		// SSO user with no stored Forta tokens — pre-checkpoint legacy state.
		// Treat as still valid; the next SSO login will populate the row.
		return true
	}
	if time.Since(sess.LastCheckedAt) < ssoCheckpointTTL {
		return true
	}

	refreshToken, err := tools.Decrypt(sess.RefreshToken)
	if err != nil {
		log.Printf("checkpointSSOGrant: decrypt refresh token: %v", err)
		return true
	}

	resp, err := sso.Introspect(refreshToken, "refresh_token")
	if err != nil {
		log.Printf("checkpointSSOGrant: introspect call failed: %v (allowing request)", err)
		return true
	}

	if !resp.Active {
		log.Printf("checkpointSSOGrant: IDP reports inactive for user %d, killing local session", userID)
		if delErr := query.DeleteSSOSession(db.DB, userID); delErr != nil {
			log.Printf("checkpointSSOGrant: failed to delete sso_session: %v", delErr)
		}
		return false
	}

	if err := query.TouchSSOSession(db.DB, userID); err != nil {
		log.Printf("checkpointSSOGrant: touch failed: %v", err)
	}
	return true
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
