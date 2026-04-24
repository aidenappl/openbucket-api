package routers

import (
	"net/http"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/jwt"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
)

func HandleRefresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("ob-refresh-token")
	if err != nil || cookie.Value == "" {
		responder.SendError(w, http.StatusUnauthorized, "no refresh token")
		return
	}

	userID, err := jwt.ValidateRefreshToken(cookie.Value)
	if err != nil {
		responder.SendError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	user, err := query.GetUserByID(db.DB, userID)
	if err != nil || user == nil || !user.Active {
		responder.SendError(w, http.StatusUnauthorized, "user not found or inactive")
		return
	}

	accessToken, accessExpiry, err := jwt.NewAccessToken(user.ID)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to generate access token")
		return
	}

	refreshToken, refreshExpiry, err := jwt.NewRefreshToken(user.ID)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}

	setTokenCookies(w, accessToken, refreshToken, accessExpiry, refreshExpiry)
	responder.New(w, user, "token refreshed")
}
