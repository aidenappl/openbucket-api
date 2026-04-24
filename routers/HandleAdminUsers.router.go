package routers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/tools"
	"github.com/gorilla/mux"
)

// HandleAdminListUsers returns all users (admin only).
func HandleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	req := query.ListUsersRequest{}

	if activeStr := r.URL.Query().Get("active"); activeStr != "" {
		active := activeStr == "true"
		req.Active = &active
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			req.Limit = limit
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			req.Offset = offset
		}
	}

	users, err := query.ListUsers(db.DB, req)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	responder.New(w, users, "users retrieved")
}

type CreateUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// HandleAdminCreateUser creates a new local user (admin only).
func HandleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var body CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		responder.SendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Email == "" || body.Password == "" {
		responder.SendError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	if err := tools.ValidateEmail(body.Email); err != nil {
		responder.SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := tools.ValidatePassword(body.Password); err != nil {
		responder.SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check for existing user
	existing, _ := query.GetUserByEmailAndAuthType(db.DB, body.Email, "local")
	if existing != nil {
		responder.SendError(w, http.StatusConflict, "user with this email already exists")
		return
	}

	role := body.Role
	if role == "" {
		role = "viewer"
	}
	validRoles := map[string]bool{"admin": true, "editor": true, "viewer": true}
	if !validRoles[role] {
		responder.SendError(w, http.StatusBadRequest, "role must be admin, editor, or viewer")
		return
	}

	hash, err := tools.HashPassword(body.Password)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	var namePtr *string
	if body.Name != "" {
		namePtr = &body.Name
	}

	user, err := query.CreateUser(db.DB, query.CreateUserRequest{
		Email:        body.Email,
		Name:         namePtr,
		AuthType:     "local",
		PasswordHash: &hash,
		Role:         role,
	})
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	responder.New(w, user, "user created")
}

type AdminUpdateUserRequest struct {
	Name   *string `json:"name"`
	Role   *string `json:"role"`
	Active *bool   `json:"active"`
}

// HandleAdminUpdateUser updates a user (admin only).
func HandleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var body AdminUpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		responder.SendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Role != nil {
		validRoles := map[string]bool{"admin": true, "editor": true, "viewer": true, "pending": true}
		if !validRoles[*body.Role] {
			responder.SendError(w, http.StatusBadRequest, "role must be admin, editor, viewer, or pending")
			return
		}
	}

	user, err := query.UpdateUser(db.DB, id, query.UpdateUserRequest{
		Name:   body.Name,
		Role:   body.Role,
		Active: body.Active,
	})
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	responder.New(w, user, "user updated")
}

// HandleAdminDeleteUser soft-deletes a user (admin only).
func HandleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	if err := query.DeleteUser(db.DB, id); err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	responder.New(w, nil, "user deactivated")
}
