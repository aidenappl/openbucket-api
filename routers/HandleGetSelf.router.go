package routers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/structs"
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
	// CurrentPassword is required with Password.
	CurrentPassword *string `json:"current_password"`
}

// checkPasswordChange validates a password change for user. It returns 0 when
// allowed, else the status and message to send.
//
// ⚠️ It used to need only a valid session: anyone holding a stolen cookie or
// an unlocked browser could set a new password and keep the account after the
// session was revoked (openbucket-api#11). It also wrote a password_hash onto
// SSO accounts, which never sign in with one.
func checkPasswordChange(user *structs.User, body UpdateSelfRequest) (int, string) {
	if user.AuthType != "local" {
		return http.StatusBadRequest, "this account signs in with SSO and has no password to change"
	}
	if body.CurrentPassword == nil || *body.CurrentPassword == "" {
		return http.StatusBadRequest, "current_password is required to change the password"
	}
	// 400, not 401: clients treat 401 as an expired session and refresh.
	if user.PasswordHash == nil || !tools.CheckPassword(*user.PasswordHash, *body.CurrentPassword) {
		return http.StatusBadRequest, "current password is incorrect"
	}
	if err := tools.ValidatePassword(*body.Password); err != nil {
		return http.StatusBadRequest, err.Error()
	}
	if *body.Password == *body.CurrentPassword {
		return http.StatusBadRequest, "the new password must differ from the current one"
	}
	return 0, ""
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
		if status, msg := checkPasswordChange(user, body); status != 0 {
			responder.SendError(w, status, msg)
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

	// A new password ends every other session: tokens issued up to now are
	// revoked, and this browser gets fresh ones so it stays signed in.
	//
	// The stamp is written from this process's clock and the new tokens are
	// issued in the next whole second: validateToken rejects any token whose
	// `iat` is not AFTER the stamp (second granularity), so tokens minted in the
	// same second — or against a database clock running ahead — would log the
	// user straight out.
	if req.PasswordHash != nil {
		cutoff := time.Now().Truncate(time.Second)
		if err := query.RevokeUserTokensAt(db.DB, int64(user.ID), cutoff); err != nil {
			responder.SendError(w, http.StatusInternalServerError, "password changed, but signing out other sessions failed")
			return
		}
		time.Sleep(time.Until(cutoff.Add(time.Second)))
		if !setAuthCookies(w, user.ID) {
			return
		}
	}

	responder.New(w, updated, "profile updated")
}
