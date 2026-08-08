package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aidenappl/openbucket-api/env"
)

func init() {
	// Tests don't go through Keyring — set values directly.
	env.CookieDomain = ""
	env.CookieInsecure = true
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestCSRF_SafeMethodsSkipValidation(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(okHandler))

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/buckets", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected 200 for %s, got %d", method, rr.Code)
			}
		})
	}
}

func TestCSRF_BearerTokenSkipsValidation(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodPost, "/buckets", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for Bearer auth, got %d", rr.Code)
	}
}

func TestCSRF_ExemptPaths(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(okHandler))

	for _, path := range []string{"/auth/login", "/auth/refresh", "/auth/sso/callback"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected 200 for exempt path %s, got %d", path, rr.Code)
			}
		})
	}
}

func TestCSRF_MissingCookie(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/buckets", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "missing CSRF cookie") {
		t.Fatalf("expected 'missing CSRF cookie' in body, got %s", rr.Body.String())
	}
}

func TestCSRF_MismatchedToken(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/buckets", nil)
	req.AddCookie(&http.Cookie{Name: "ob-csrf", Value: "cookie-token-abc"})
	req.Header.Set("X-CSRF-Token", "different-token-xyz")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "CSRF token mismatch") {
		t.Fatalf("expected 'CSRF token mismatch' in body, got %s", rr.Body.String())
	}
}

func TestCSRF_ValidDoubleSubmit(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(okHandler))

	token := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4"
	req := httptest.NewRequest(http.MethodPost, "/buckets", nil)
	req.AddCookie(&http.Cookie{Name: "ob-csrf", Value: token})
	req.Header.Set("X-CSRF-Token", token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCSRF_MissingHeader(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/buckets", nil)
	req.AddCookie(&http.Cookie{Name: "ob-csrf", Value: "some-token"})
	// No X-CSRF-Token header
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestCSRF_SetsCookieOnGET(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodGet, "/buckets", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should set an ob-csrf cookie for new requests
	found := false
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == "ob-csrf" {
			found = true
			if cookie.Value == "" {
				t.Fatal("CSRF cookie should not be empty")
			}
			if cookie.HttpOnly {
				t.Fatal("CSRF cookie should not be HttpOnly (JS must read it)")
			}
		}
	}
	if !found {
		t.Fatal("expected ob-csrf cookie to be set")
	}
}

func TestCSRF_EmptyCookieValue(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/buckets", nil)
	req.AddCookie(&http.Cookie{Name: "ob-csrf", Value: ""})
	req.Header.Set("X-CSRF-Token", "")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

// TestCSRF_ExemptsBackchannelLogout guards against a failure that is invisible
// from both ends.
//
// ─────────────────────────────────────────────────────────────────────────────
// The back-channel logout notification is a server-to-server POST from the
// identity provider: no cookie, no Bearer token, no custom header. Without an
// exemption it falls through to the double-submit check it can never satisfy and
// is refused 403 "missing CSRF cookie". The provider then retries six times,
// marks the delivery exhausted, and revocation silently stays at poll speed —
// while the endpoint looks like a broken receiver rather than a blocked one.
//
// monitor-core shipped exactly that on 2026-08-08. Its routing test passed
// throughout, because the router was never the thing rejecting the request. This
// test exists here so openbucket-api does not repeat it.
// ─────────────────────────────────────────────────────────────────────────────
func TestCSRF_ExemptsBackchannelLogout(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodPost, "/auth/sso/backchannel-logout",
		strings.NewReader("logout_token=signed.jwt.here"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("CSRF refused the back-channel logout POST (status %d, body %s).\n\n"+
			"The provider sends this with no cookie and no auth headers, so without the "+
			"exemption every notification is rejected, retried to exhaustion and marked "+
			"exhausted — revocation quietly stays at poll speed.", rr.Code, strings.TrimSpace(rr.Body.String()))
	}
}

// TestCSRF_ExemptionIsNarrow keeps the exemption from becoming a bypass.
//
// ⚠️ The failure guarded against is a predicate that is too loose — a
// strings.Contains, or a prefix swallowing the whole /auth/sso/ tree. That would
// hand a CSRF bypass to real endpoints while every back-channel test still passed.
func TestCSRF_ExemptionIsNarrow(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(okHandler))

	for _, path := range []string{
		"/auth/sso/backchannel-logout/extra",
		"/auth/sso/backchannel-logout-something",
		"/auth/sso/config",
		"/buckets",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code == http.StatusOK {
				t.Errorf("POST %s passed CSRF with no cookie and no token — the back-channel "+
					"exemption is too broad and is now a CSRF bypass", path)
			}
		})
	}
}
