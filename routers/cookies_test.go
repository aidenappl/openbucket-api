package routers

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aidenappl/openbucket-api/env"
)

func init() {
	// Override env for tests — no Keyring required.
	env.JWTSigningKey = "test-signing-key-must-be-at-least-32-chars-long"
	env.CookieDomain = ""
	env.CookieInsecure = true
}

func TestSetTokenCookies(t *testing.T) {
	rr := httptest.NewRecorder()

	accessExpiry := time.Now().Add(15 * time.Minute)
	refreshExpiry := time.Now().Add(7 * 24 * time.Hour)

	setTokenCookies(rr, "access-tok-abc", "refresh-tok-xyz", accessExpiry, refreshExpiry)

	cookies := rr.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("expected 3 cookies, got %d", len(cookies))
	}

	cookieMap := map[string]*struct {
		value    string
		httpOnly bool
	}{}
	for _, c := range cookies {
		cookieMap[c.Name] = &struct {
			value    string
			httpOnly bool
		}{c.Value, c.HttpOnly}
	}

	// ob-access-token
	if c, ok := cookieMap["ob-access-token"]; !ok {
		t.Fatal("missing ob-access-token cookie")
	} else {
		if c.value != "access-tok-abc" {
			t.Fatalf("expected access token value, got %s", c.value)
		}
		if !c.httpOnly {
			t.Fatal("ob-access-token should be HttpOnly")
		}
	}

	// ob-refresh-token
	if c, ok := cookieMap["ob-refresh-token"]; !ok {
		t.Fatal("missing ob-refresh-token cookie")
	} else {
		if c.value != "refresh-tok-xyz" {
			t.Fatalf("expected refresh token value, got %s", c.value)
		}
		if !c.httpOnly {
			t.Fatal("ob-refresh-token should be HttpOnly")
		}
	}

	// logged_in
	if c, ok := cookieMap["logged_in"]; !ok {
		t.Fatal("missing logged_in cookie")
	} else {
		if c.value != "true" {
			t.Fatalf("expected logged_in=true, got %s", c.value)
		}
		if c.httpOnly {
			t.Fatal("logged_in should NOT be HttpOnly (frontend JS reads it)")
		}
	}
}

func TestSetTokenCookies_SecureFlag(t *testing.T) {
	// When CookieInsecure is false, cookies should be Secure
	env.CookieInsecure = false
	defer func() { env.CookieInsecure = true }()

	rr := httptest.NewRecorder()
	setTokenCookies(rr, "tok", "ref", time.Now().Add(time.Hour), time.Now().Add(time.Hour))

	for _, c := range rr.Result().Cookies() {
		if !c.Secure {
			t.Fatalf("cookie %s should be Secure when CookieInsecure=false", c.Name)
		}
	}
}

func TestSetAuthCookies_Success(t *testing.T) {
	rr := httptest.NewRecorder()
	ok := setAuthCookies(rr, 42)
	if !ok {
		t.Fatal("expected setAuthCookies to return true")
	}

	cookies := rr.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("expected 3 cookies, got %d", len(cookies))
	}

	// Verify tokens are non-empty JWTs
	for _, c := range cookies {
		if c.Value == "" {
			t.Fatalf("cookie %s has empty value", c.Name)
		}
	}
}
