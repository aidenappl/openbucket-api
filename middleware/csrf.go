package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/aidenappl/openbucket-api/env"
)

const csrfCookieName = "ob-csrf"
const csrfHeaderName = "X-CSRF-Token"

// CSRFMiddleware implements the double-submit cookie pattern for CSRF protection.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ensure CSRF cookie exists
		if _, err := r.Cookie(csrfCookieName); err != nil {
			setCSRFCookie(w)
		}

		// Skip validation for safe methods
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Skip for Bearer token auth (stateless API clients)
		if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip exempt paths.
		//
		// ⚠️ /auth/sso/backchannel-logout IS NOT OPTIONAL HERE. It is a
		// server-to-server POST from the identity provider (OIDC Back-Channel
		// Logout 1.0 §2.5) carrying no cookie, no Bearer token and no custom
		// header, so without this line it falls through to the double-submit check
		// it can never satisfy and is refused 403 "missing CSRF cookie". The
		// provider then retries six times, marks the delivery exhausted, and
		// revocation silently stays at poll speed while the endpoint looks like a
		// broken receiver. monitor-core shipped exactly that on 2026-08-08 and it
		// took a live probe to find, because its routing test passed throughout.
		//
		// Exempting is correct rather than a concession: CSRF defends AMBIENT
		// COOKIE AUTHORITY, and this endpoint has none — it reads no cookie and no
		// session, and its sole authentication is the signature on the logout
		// token, verified against the provider's JWKS. That is strictly stronger
		// than a double-submit cookie, so the check can only reject the legitimate
		// caller.
		path := r.URL.Path
		if path == "/auth/login" || path == "/auth/refresh" || path == "/auth/sso/callback" ||
			path == "/auth/sso/backchannel-logout" {
			next.ServeHTTP(w, r)
			return
		}

		// Validate
		cookie, err := r.Cookie(csrfCookieName)
		if err != nil || cookie.Value == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":null,"error_message":"missing CSRF cookie","error_code":4030}`))
			return
		}
		headerToken := r.Header.Get(csrfHeaderName)
		if headerToken == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(headerToken)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":null,"error_message":"CSRF token mismatch","error_code":4031}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func setCSRFCookie(w http.ResponseWriter) {
	token := generateCSRFToken()
	secure := !env.CookieInsecure

	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		Domain:   env.CookieDomain,
		HttpOnly: false, // JavaScript needs to read this
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
