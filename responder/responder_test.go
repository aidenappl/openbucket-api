package responder

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNew_DefaultMessage(t *testing.T) {
	rr := httptest.NewRecorder()
	New(rr, map[string]string{"key": "value"})

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var resp ResponseStructure
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Message != "request was successful" {
		t.Fatalf("expected default message, got %q", resp.Message)
	}
}

func TestNew_CustomMessage(t *testing.T) {
	rr := httptest.NewRecorder()
	New(rr, nil, "Login Successful")

	var resp ResponseStructure
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Message != "login successful" {
		t.Fatalf("expected lowercased message, got %q", resp.Message)
	}
}

func TestNew_NilData(t *testing.T) {
	rr := httptest.NewRecorder()
	New(rr, nil)

	var resp ResponseStructure
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
}

func TestNew_DataPreserved(t *testing.T) {
	rr := httptest.NewRecorder()

	type item struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	data := []item{{ID: 1, Name: "alpha"}, {ID: 2, Name: "beta"}}
	New(rr, data)

	var resp struct {
		Success bool   `json:"success"`
		Data    []item `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Data))
	}
	if resp.Data[0].Name != "alpha" {
		t.Fatalf("data mismatch: %+v", resp.Data)
	}
}

func TestSendError_Basic(t *testing.T) {
	rr := httptest.NewRecorder()
	SendError(rr, http.StatusBadRequest, "bad input")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.ErrorMessage != "bad input" {
		t.Fatalf("expected 'bad input', got %q", resp.ErrorMessage)
	}
}

func TestSendError_StatusCodes(t *testing.T) {
	tests := []struct {
		status int
	}{
		{http.StatusBadRequest},
		{http.StatusUnauthorized},
		{http.StatusForbidden},
		{http.StatusNotFound},
		{http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			rr := httptest.NewRecorder()
			SendError(rr, tt.status, "test error")

			if rr.Code != tt.status {
				t.Fatalf("expected %d, got %d", tt.status, rr.Code)
			}

			var resp ErrorResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode: %v", err)
			}
			if resp.ErrorMessage != "test error" {
				t.Fatalf("expected 'test error', got %q", resp.ErrorMessage)
			}
		})
	}
}

func TestSendError_DoesNotLeakInternalErrors(t *testing.T) {
	rr := httptest.NewRecorder()
	SendError(rr, http.StatusInternalServerError, "something went wrong", nil)

	body := rr.Body.String()
	// The internal error details should NOT appear in the response body
	if strings.Contains(body, "sql:") || strings.Contains(body, "panic") {
		t.Fatal("response body should not contain internal error details")
	}
}

func TestErrMissingParam(t *testing.T) {
	rr := httptest.NewRecorder()
	ErrMissingParam(rr, "bucket_id")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if !strings.Contains(resp.ErrorMessage, "bucket_id") {
		t.Fatalf("expected error message to contain field name, got %q", resp.ErrorMessage)
	}
}

func TestErrMissingKey(t *testing.T) {
	rr := httptest.NewRecorder()
	ErrMissingKey(rr, "name")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if !strings.Contains(resp.ErrorMessage, "name") {
		t.Fatalf("expected error message to contain key name, got %q", resp.ErrorMessage)
	}
}

func TestSendErrorWithParams(t *testing.T) {
	rr := httptest.NewRecorder()
	code := 4004
	msg := "account pending"
	SendErrorWithParams(rr, "pending_approval", http.StatusForbidden, &code, &msg)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.ErrorCode != 4004 {
		t.Fatalf("expected error_code 4004, got %d", resp.ErrorCode)
	}
	if resp.ErrorMessage != "account pending" {
		t.Fatalf("expected 'account pending', got %q", resp.ErrorMessage)
	}
}
