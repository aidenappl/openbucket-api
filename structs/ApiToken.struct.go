package structs

import "time"

// ApiToken is a long-lived opaque credential used by automation (CLIs, CI, MCP
// servers) in place of the short-lived access/refresh JWT pair. Only the
// SHA-256 hash is persisted; the plaintext is shown once at creation.
type ApiToken struct {
	ID         int        `json:"id"`
	UserID     int        `json:"user_id"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"-"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	Active     bool       `json:"active"`
	UpdatedAt  time.Time  `json:"updated_at"`
	InsertedAt time.Time  `json:"inserted_at"`
}

type CreateApiTokenRequest struct {
	Name      string `json:"name"`
	ExpiresIn string `json:"expires_in"`
}

// CreateApiTokenResponse embeds the created record alongside the plaintext
// token, which is returned only on creation and never retrievable again.
type CreateApiTokenResponse struct {
	ApiToken
	Token string `json:"token"`
}
