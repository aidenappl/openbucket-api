package routers

import (
	"net/http"
	"time"

	"github.com/aidenappl/openbucket-api/env"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/sso"
)

func HandleLogout(w http.ResponseWriter, r *http.Request) {
	secure := !env.CookieInsecure

	// If this is an SSO user, drop the persisted provider tokens too.
	// An SSO session ends here and, through RP-initiated logout, at the
	// identity provider too — which back-channels every other application.
	// The frontend navigates to redirect_url to complete that.
	redirectURL := ""
	if sso.SessionRequest(r) {
		redirectURL = sso.Manager().Logout(w, r)
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

	responder.New(w, map[string]string{"redirect_url": redirectURL}, "logged out")
}
