package routers

import (
	"log"
	"net/http"
	"time"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/env"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
)

func HandleLogout(w http.ResponseWriter, r *http.Request) {
	secure := !env.CookieInsecure

	// If this is an SSO user, drop the persisted Forta tokens too.
	if user, ok := middleware.GetUserFromContext(r.Context()); ok && user != nil && user.AuthType == "sso" {
		if err := query.DeleteSSOSession(db.DB, int64(user.ID)); err != nil {
			log.Printf("HandleLogout: failed to delete sso_session for user %d: %v", user.ID, err)
		}
	}

	// Clear all auth cookies
	for _, name := range []string{"ob-access-token", "ob-refresh-token", "ob-logged-in"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Domain:   env.CookieDomain,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: name != "ob-logged-in",
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
	}

	responder.New(w, nil, "logged out")
}
