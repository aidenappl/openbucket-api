package structs

import "time"

type User struct {
	ID              int     `json:"id"`
	Email           string  `json:"email"`
	Name            *string `json:"name"`
	AuthType        string  `json:"auth_type"`
	PasswordHash    *string `json:"-"`
	SSOSubject      *string `json:"sso_subject,omitempty"`
	ProfileImageURL *string `json:"profile_image_url"`
	Role            string  `json:"role"`
	Active          bool    `json:"active"`

	// TokensRevokedAt is the cut-off for this user's own OpenBucket JWTs: any token
	// issued before it is rejected. NULL means nothing has been revoked.
	//
	// ⚠️ IT IS WHAT MAKES SSO REVOCATION STICK. Deleting the sso_sessions row does
	// not lock anyone out — OpenBucket validates its own JWTs locally with no
	// reference to that row, and the next request would find no row and take the
	// "not an SSO session, allow" branch. See middleware.validateToken.
	TokensRevokedAt *time.Time `json:"-"`
	UpdatedAt       time.Time  `json:"updated_at"`
	InsertedAt      time.Time  `json:"inserted_at"`
}
