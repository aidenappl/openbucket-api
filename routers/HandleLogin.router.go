package routers

import (
	"encoding/json"
	"net/http"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/jwt"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/tools"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	var body LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		responder.SendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Email == "" || body.Password == "" {
		responder.SendError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := query.GetUserByEmailAndAuthType(db.DB, body.Email, "local")
	if err != nil || user == nil {
		responder.SendError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !user.Active {
		responder.SendError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if user.PasswordHash == nil || !tools.CheckPassword(*user.PasswordHash, body.Password) {
		responder.SendError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !setAuthCookies(w, user.ID) {
		return
	}
	responder.New(w, user, "login successful")
}

func setAuthCookies(w http.ResponseWriter, userID int) bool {
	accessToken, accessExpiry, err := jwt.NewAccessToken(userID)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to generate access token")
		return false
	}

	refreshToken, refreshExpiry, err := jwt.NewRefreshToken(userID)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return false
	}

	setTokenCookies(w, accessToken, refreshToken, accessExpiry, refreshExpiry)
	return true
}
