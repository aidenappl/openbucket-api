package routers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleLogin_MissingBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader("not json"))
	rr := httptest.NewRecorder()

	HandleLogin(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleLogin_EmptyFields(t *testing.T) {
	tests := []struct {
		name  string
		email string
		pass  string
	}{
		{"missing both", "", ""},
		{"missing email", "", "password123"},
		{"missing password", "user@example.com", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(LoginRequest{Email: tt.email, Password: tt.pass})
			req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
			rr := httptest.NewRecorder()

			HandleLogin(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", rr.Code)
			}

			var resp map[string]any
			json.Unmarshal(rr.Body.Bytes(), &resp)
			if msg, ok := resp["error_message"].(string); !ok || msg == "" {
				t.Fatal("expected error_message in response")
			}
		})
	}
}

func TestHandleLogin_ResponseFormat(t *testing.T) {
	// Validates that even error responses are valid JSON
	body, _ := json.Marshal(LoginRequest{Email: "", Password: ""})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	HandleLogin(rr, req)

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected JSON content-type, got %s", ct)
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
}
