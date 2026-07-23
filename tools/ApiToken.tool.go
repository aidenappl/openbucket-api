package tools

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// ApiTokenPrefix marks a credential as an opaque OpenBucket API token rather
// than a JWT. The prefix lets middleware pick a validation path without
// parsing, and gives secret scanners a stable pattern to match on.
const ApiTokenPrefix = "obk_"

// GenerateApiToken returns a new plaintext API token and its SHA-256 hex
// digest. Only the hash is ever persisted.
func GenerateApiToken() (plaintext string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate random token: %w", err)
	}
	plaintext = ApiTokenPrefix + hex.EncodeToString(b)
	return plaintext, HashApiToken(plaintext), nil
}

// HashApiToken returns the SHA-256 hex digest of a plaintext API token.
func HashApiToken(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(h[:])
}

// IsApiToken reports whether a bearer credential is an opaque API token.
func IsApiToken(token string) bool {
	return strings.HasPrefix(token, ApiTokenPrefix)
}
