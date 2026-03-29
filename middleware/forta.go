package middleware

import (
	"context"
	"net/http"

	forta "github.com/aidenappl/go-forta"
)

// FortaUserContextKey is the key used to store the forta user in context
const FortaUserContextKey contextKey = "fortaUser"

// FortaIDContextKey is the key used to store the forta user ID in context
const FortaIDContextKey contextKey = "fortaID"

// FortaProtected wraps a handler with Forta authentication.
// Use this to protect routes that require a valid Forta user session.
// The handler will receive the Forta user ID and optionally the full user profile in context.
func FortaProtected(next http.HandlerFunc) http.HandlerFunc {
	return forta.Protected(next)
}

// GetFortaID retrieves the Forta user ID from the request context.
// Returns the user ID and true if available, or 0 and false if not.
// This is always available inside a FortaProtected handler.
func GetFortaID(ctx context.Context) (int64, bool) {
	return forta.GetFortaIDFromContext(ctx)
}

// GetFortaUser retrieves the full Forta user profile from the request context.
// Returns the user and true if available, or nil and false if not.
// This is only available when:
//   - JWTSigningKey is empty (remote /oauth/userinfo validation), OR
//   - FetchUserOnProtect: true
func GetFortaUser(ctx context.Context) (*forta.User, bool) {
	return forta.GetUserFromContext(ctx)
}

// FetchCurrentUser retrieves the full Forta user profile from the current request.
// This can be used in any handler (not just FortaProtected ones) to check if a user is authenticated.
// Returns nil and an error if no token is present or the token is invalid.
// Note: This does NOT perform auto-refresh or set response cookies.
func FetchCurrentUser(r *http.Request) (*forta.User, error) {
	return forta.FetchCurrentUser(r)
}
