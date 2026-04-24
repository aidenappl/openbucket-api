package routers

import (
	"net/http"
	"time"

	"github.com/aidenappl/openbucket-api/env"
	"github.com/aidenappl/openbucket-api/responder"
)

func HandleLogout(w http.ResponseWriter, r *http.Request) {
	secure := !env.CookieInsecure

	// Clear all auth cookies
	for _, name := range []string{"ob-access-token", "ob-refresh-token", "logged_in"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Domain:   env.CookieDomain,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: name != "logged_in",
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
	}

	responder.New(w, nil, "logged out")
}
