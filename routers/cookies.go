package routers

import (
	"net/http"
	"time"

	"github.com/aidenappl/openbucket-api/env"
)

func setTokenCookies(w http.ResponseWriter, accessToken, refreshToken string, accessExpiry, refreshExpiry time.Time) {
	secure := !env.CookieInsecure

	http.SetCookie(w, &http.Cookie{
		Name:     "ob-access-token",
		Value:    accessToken,
		Path:     "/",
		Domain:   env.CookieDomain,
		Expires:  accessExpiry,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "ob-refresh-token",
		Value:    refreshToken,
		Path:     "/",
		Domain:   env.CookieDomain,
		Expires:  refreshExpiry,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	// Non-HttpOnly cookie for frontend JS to detect login state
	http.SetCookie(w, &http.Cookie{
		Name:     "logged_in",
		Value:    "true",
		Path:     "/",
		Domain:   env.CookieDomain,
		Expires:  refreshExpiry,
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
