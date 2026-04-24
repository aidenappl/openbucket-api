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
