package routers

import (
	"encoding/json"
	"net/http"

	forta "github.com/aidenappl/go-forta"
	"github.com/aidenappl/openbucket-api/responder"
)

// CurrentUserResponse represents the response for the current user endpoint
type CurrentUserResponse struct {
	ID          int64   `json:"id"`
	Email       string  `json:"email,omitempty"`
	Name        *string `json:"name,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
}

// HandleGetCurrentUser returns the currently authenticated Forta user's information.
// This endpoint requires Forta authentication (use forta.Protected wrapper).
func HandleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	// Get the Forta user ID (always available in Protected handlers)
	fortaID, ok := forta.GetFortaIDFromContext(r.Context())
	if !ok {
		responder.SendError(w, http.StatusUnauthorized, "unauthenticated", nil)
		return
	}

	response := CurrentUserResponse{
		ID: fortaID,
	}

	// Get the full user profile if available
	// (only present when JWTSigningKey is empty OR FetchUserOnProtect is true)
	user, hasUser := forta.GetUserFromContext(r.Context())
	if hasUser {
		response.Email = user.Email
		response.Name = user.Name
		response.DisplayName = user.DisplayName
	}

	responder.New(w, response, "successfully retrieved current user")
}

// HandleCheckAuth checks if the user is authenticated without requiring authentication.
// Returns user info if authenticated, or a guest response if not.
func HandleCheckAuth(w http.ResponseWriter, r *http.Request) {
	user, err := forta.FetchCurrentUser(r)
	if err != nil {
		// Not authenticated - return guest response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": false,
			"message":       "not authenticated",
		})
		return
	}

	// Authenticated - return user info
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": true,
		"user": CurrentUserResponse{
			ID:          user.ID,
			Email:       user.Email,
			Name:        user.Name,
			DisplayName: user.DisplayName,
		},
	})
}
