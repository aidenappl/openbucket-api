package jwt

import (
	"testing"
	"time"

	"github.com/aidenappl/openbucket-api/env"
)

func TestMain(m *testing.M) {
	env.JWTSigningKey = "test-signing-key-must-be-at-least-32-chars-long"
	m.Run()
}

func TestNewAccessToken(t *testing.T) {
	token, expiry, err := NewAccessToken(42)
	if err != nil {
		t.Fatalf("NewAccessToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if expiry.Before(time.Now()) {
		t.Fatal("expected expiry in the future")
	}
	if expiry.After(time.Now().Add(16 * time.Minute)) {
		t.Fatal("expected expiry within ~15 minutes")
	}
}

func TestNewRefreshToken(t *testing.T) {
	token, expiry, err := NewRefreshToken(42)
	if err != nil {
		t.Fatalf("NewRefreshToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if expiry.Before(time.Now().Add(6 * 24 * time.Hour)) {
		t.Fatal("expected expiry at least 6 days out")
	}
}

func TestValidateAccessToken(t *testing.T) {
	token, _, _ := NewAccessToken(99)

	userID, err := ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}
	if userID != 99 {
		t.Fatalf("expected userID 99, got %d", userID)
	}
}

func TestValidateRefreshToken(t *testing.T) {
	token, _, _ := NewRefreshToken(77)

	userID, err := ValidateRefreshToken(token)
	if err != nil {
		t.Fatalf("ValidateRefreshToken failed: %v", err)
	}
	if userID != 77 {
		t.Fatalf("expected userID 77, got %d", userID)
	}
}

func TestAccessTokenRejectsRefresh(t *testing.T) {
	token, _, _ := NewRefreshToken(1)

	_, err := ValidateAccessToken(token)
	if err == nil {
		t.Fatal("expected error when validating refresh token as access token")
	}
}

func TestRefreshTokenRejectsAccess(t *testing.T) {
	token, _, _ := NewAccessToken(1)

	_, err := ValidateRefreshToken(token)
	if err == nil {
		t.Fatal("expected error when validating access token as refresh token")
	}
}

func TestInvalidToken(t *testing.T) {
	_, err := ValidateAccessToken("garbage.token.here")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestWrongSigningKey(t *testing.T) {
	token, _, _ := NewAccessToken(1)

	// Change the signing key
	env.JWTSigningKey = "different-key-that-is-also-32-chars-long"
	_, err := ValidateAccessToken(token)
	if err == nil {
		t.Fatal("expected error for wrong signing key")
	}

	// Restore
	env.JWTSigningKey = "test-signing-key-must-be-at-least-32-chars-long"
}
