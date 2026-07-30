package sso

import (
	"strings"

	ssolib "github.com/aidenappl/go-forta/sso"
)

// ProviderSlug is the single provider identifier this service uses.
//
// ⚠️ OpenBucket remains SINGLE-PROVIDER, matching lattice-api and for the same
// reason: an sso_providers table changes the admin API's request and response
// shapes, so openbucket-web and openbucket-mcp change with it — and the per-site
// controls those need are Phase 7's subject. Protocol correctness lands now on the
// existing single-config model; the table lands with the UI that configures it.
const ProviderSlug = "sso"

// Provider maps the stored SSOConfig onto the library's provider view.
//
// KindOAuth2: OpenBucket configures explicit endpoint URLs rather than an issuer,
// so there is no discovery document and no id_token.
//
// ⚠️ THAT IS A REAL LIMITATION. With no id_token there is no signed assertion of
// identity — the subject comes from a bearer-token UserInfo call, so anything that
// can obtain an access token can become that user. The upgrade is one config field
// (an issuer URL) plus KindOIDC; forta-api has published a conforming discovery
// document since Phase 1. PKCE applies either way and is enforced by the library.
func (c *SSOConfig) Provider() *ssolib.Provider {
	return &ssolib.Provider{
		Slug:        ProviderSlug,
		DisplayName: c.ButtonLabel,
		Kind:        ssolib.KindOAuth2,

		AuthorizeURL:  c.AuthorizeURL,
		TokenURL:      c.TokenURL,
		UserInfoURL:   c.UserInfoURL,
		IntrospectURL: c.IntrospectURL,

		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,

		Scopes:      c.Scopes,
		RedirectURL: c.RedirectURL,

		// ⚠️ DELIBERATELY EMPTY, so the library reads the standard `sub`.
		//
		// It is NOT wired to sso.user_identifier. That setting named the claim treated
		// as the identity and defaulted to "email"; the old GetUserIdentifier read it,
		// fell back to email regardless, and the result was stored as the subject.
		// Identity keyed on a reassignable address is an account-takeover primitive.
		// user_identifier is retained in the admin API for compatibility and is read
		// by nothing. Do not reconnect it.
		SubjectClaim: "",

		AllowAutoLink: false,
		AutoProvision: c.AutoProvision,

		// forta-api asserts no email_verified claim, and it is a provider we operate
		// that verifies addresses itself. This is a statement about that provider, not
		// about any user.
		TrustEmailVerified: true,
	}
}

// LooksLikeEmail reports whether a stored subject is actually an email address.
//
// It recognises rows written by the old GetUserIdentifier so the callback can heal
// them on the next login instead of requiring a backfill. A real `sub` from
// forta-api is a UUID and can never contain "@", so the test cannot misfire on a
// legitimate value.
func LooksLikeEmail(subject string) bool {
	return strings.Contains(subject, "@")
}
