package structs

import "time"

// SSOSession holds the IDP tokens for a user who logged in via SSO, plus
// the timestamp of the last successful introspection. Tokens are stored
// encrypted at rest via tools.Encrypt/Decrypt.
type SSOSession struct {
	UserID   int64  `json:"user_id"`
	Provider string `json:"provider"`

	// Subject and SID are how a back-channel logout finds this session — a logout
	// token names one or the other.
	//
	// ⚠️ Both nulls are NORMAL. Subject is null on rows written before migration
	// 010. SID is null for those AND for every session established while
	// sso.issuer_url is unset, because the OAuth2 adapter has no id_token to take
	// one from. Neither can be backfilled: `sid` lives only in the id_token of the
	// login that created the row.
	Subject *string `json:"-"`
	SID     *string `json:"-"`

	AccessToken   string    `json:"-"`
	RefreshToken  string    `json:"-"`
	LastCheckedAt time.Time `json:"last_checked_at"`
	InsertedAt    time.Time `json:"inserted_at"`
}
