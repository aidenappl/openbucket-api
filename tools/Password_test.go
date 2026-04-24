package tools

import "testing"

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("testpassword123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == "testpassword123" {
		t.Fatal("hash should not equal plaintext")
	}
}

func TestCheckPassword(t *testing.T) {
	hash, _ := HashPassword("mypassword")

	if !CheckPassword(hash, "mypassword") {
		t.Fatal("expected password to match")
	}

	if CheckPassword(hash, "wrongpassword") {
		t.Fatal("expected wrong password to not match")
	}
}

func TestDifferentHashesForSamePassword(t *testing.T) {
	hash1, _ := HashPassword("samepassword")
	hash2, _ := HashPassword("samepassword")

	if hash1 == hash2 {
		t.Fatal("expected different hashes for same password (different salts)")
	}

	// But both should validate
	if !CheckPassword(hash1, "samepassword") {
		t.Fatal("hash1 should match")
	}
	if !CheckPassword(hash2, "samepassword") {
		t.Fatal("hash2 should match")
	}
}
