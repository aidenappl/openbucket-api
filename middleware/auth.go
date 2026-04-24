package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/jwt"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/structs"
)

type contextKey string

const (
	UserContextKey   contextKey = "user"
	obAccessToken               = "ob-access-token"
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
