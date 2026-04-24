package tools

import "testing"

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "test@example.com", false},
		{"valid with dots", "first.last@example.com", false},
		{"valid with plus", "user+tag@example.com", false},
		{"no @", "testexample.com", true},
		{"no domain", "test@", true},
		{"no user", "@example.com", true},
		{"empty", "", true},
		{"no tld", "test@example", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"valid 8 chars", "password", false},
		{"valid long", "a-very-long-and-secure-password-123!", false},
		{"too short", "short", true},
		{"empty", "", true},
		{"exactly 8", "12345678", false},
		{"exactly 7", "1234567", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword(%q) error = %v, wantErr %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestValidateExternalURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://example.com/callback", false},
		{"empty", "", true},
		{"http not allowed", "http://example.com/callback", true},
		{"ftp not allowed", "ftp://example.com/file", true},
		{"localhost blocked", "https://localhost/callback", true},
		{"dot local blocked", "https://app.local/callback", true},
		{"no host", "https:///path", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExternalURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExternalURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}
