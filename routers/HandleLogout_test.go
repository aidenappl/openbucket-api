package routers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleLogout_ClearsCookies(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rr := httptest.NewRecorder()

	HandleLogout(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	cookies := rr.Result().Cookies()
	expectedNames := map[string]bool{
		"ob-access-token":  false,
		"ob-refresh-token": false,
		"logged_in":        false,
	}

	for _, c := range cookies {
		if _, ok := expectedNames[c.Name]; ok {
			expectedNames[c.Name] = true
			if c.MaxAge != -1 {
				t.Fatalf("cookie %s should have MaxAge=-1 to clear, got %d", c.Name, c.MaxAge)
			}
			if c.Value != "" {
				t.Fatalf("cookie %s should have empty value, got %s", c.Name, c.Value)
			}
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Fatalf("expected cookie %s to be cleared", name)
		}
	}
}

func TestHandleLogout_LoggedInNotHttpOnly(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rr := httptest.NewRecorder()

	HandleLogout(rr, req)

	for _, c := range rr.Result().Cookies() {
		if c.Name == "logged_in" && c.HttpOnly {
			t.Fatal("logged_in cookie should not be HttpOnly")
		}
		if c.Name == "ob-access-token" && !c.HttpOnly {
			t.Fatal("ob-access-token cookie should be HttpOnly")
		}
		if c.Name == "ob-refresh-token" && !c.HttpOnly {
			t.Fatal("ob-refresh-token cookie should be HttpOnly")
		}
	}
}
