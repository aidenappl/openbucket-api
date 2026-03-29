package routers

import (
	"net/http"

	forta "github.com/aidenappl/go-forta"
	"github.com/aidenappl/openbucket-api/responder"
)

// HandleGetCurrentUser returns the currently authenticated Forta user's information.
// This endpoint requires Forta authentication (use forta.Protected wrapper).
func HandleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	// Get the Forta user ID (always available in Protected handlers)
	_, ok := forta.GetFortaIDFromContext(r.Context())
	if !ok {
		responder.SendError(w, http.StatusUnauthorized, "unauthenticated", nil)
		return
	}

	// Try context first (populated if FetchUserOnProtect or remote validation)
	user, hasUser := forta.GetUserFromContext(r.Context())
	if !hasUser {
		// Explicitly fetch the full profile from Forta API
		fetched, err := forta.FetchCurrentUser(r)
		if err != nil {
			responder.SendError(w, http.StatusInternalServerError, "failed to fetch user profile", err)
			return
		}
		user = fetched
	}

	responder.New(w, user, "successfully retrieved current user")
}
