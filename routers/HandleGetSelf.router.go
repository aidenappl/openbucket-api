package routers

import (
	"encoding/json"
	"net/http"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/tools"
)

// HandleGetSelf returns the currently authenticated user.
func HandleGetSelf(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok || user == nil {
		responder.SendError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	responder.New(w, user, "successfully retrieved current user")
}

type UpdateSelfRequest struct {
	Name     *string `json:"name"`
	Password *string `json:"password"`
}

// HandleUpdateSelf updates the currently authenticated user's profile.
func HandleUpdateSelf(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok || user == nil {
		responder.SendError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var body UpdateSelfRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		responder.SendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req := query.UpdateUserRequest{}
	if body.Name != nil {
		req.Name = body.Name
	}
	if body.Password != nil {
		if err := tools.ValidatePassword(*body.Password); err != nil {
			responder.SendError(w, http.StatusBadRequest, err.Error())
			return
		}
		hash, err := tools.HashPassword(*body.Password)
		if err != nil {
			responder.SendError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}
		req.PasswordHash = &hash
	}

	updated, err := query.UpdateUser(db.DB, user.ID, req)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	responder.New(w, updated, "profile updated")
}
