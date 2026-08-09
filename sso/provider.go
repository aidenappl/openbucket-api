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
// ⚠️ THE KIND IS CHOSEN BY WHETHER AN ISSUER IS CONFIGURED, and the difference is
// a security one rather than a matter of taste.
//
//   - IssuerURL set  → KindOIDC. The library discovers the provider's metadata,
//     verifies a SIGNED id_token, checks the nonce, and surfaces `sid` — which is
//     the only handle by which a back-channel logout can name one session.
//   - IssuerURL empty → KindOAuth2, the legacy path. There is no id_token at all,
//     so identity arrives from a bearer-token UserInfo call signed by nothing:
//     anything that can obtain an access token can become that user.
//
// The fallback exists so an existing deployment keeps working across the upgrade,
// NOT because the two are equivalent. Set sso.issuer_url and this becomes strictly
// stronger; forta-api has published a conforming discovery document since Phase 1.
// PKCE applies either way and is enforced by the library.
func (c *SSOConfig) Provider() *ssolib.Provider {
	return &ssolib.Provider{
		Slug:        ProviderSlug,
		DisplayName: c.ButtonLabel,
		Kind:        c.kind(),
		IssuerURL:   c.IssuerURL,

		// ⚠️ REQUIRED UNDER OIDC, and not merely an optimisation.
		//
		// forta-api's id_token deliberately carries NO email claim — OIDC Core §2
		// requires `sub` to be stable and never reassigned, and Forta declines to
		// put a reassignable address next to it. But this service REFUSES a login
		// with no email (sso_no_email), because it keys local accounts on one.
		//
		// Under KindOAuth2 that was invisible: UserInfo was the only identity
		// source, so email always arrived. Under KindOIDC the adapter reads the
		// id_token and calls UserInfo ONLY when this is set — so switching to OIDC
		// without it breaks every login with "the SSO provider did not return an
		// email address". lattice-api hit exactly that on 2026-08-09.
		//
		// The extra round trip is the price of an email this provider will not put
		// in a signed token. The library still enforces the OIDC Core §5.3.2 `sub`
		// check on the response, so an unsigned UserInfo cannot change who you are —
		// it can only add claims the id_token did not carry.
		FetchUserInfo: true,

		// Ignored by the OIDC adapter, which discovers them. Kept populated so
		// clearing the issuer falls back cleanly instead of to a half-configured
		// provider that fails at the first redirect.
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

// kind reports which adapter this configuration should use. See Provider.
func (c *SSOConfig) kind() ssolib.Kind {
	if strings.TrimSpace(c.IssuerURL) != "" {
		return ssolib.KindOIDC
	}
	return ssolib.KindOAuth2
}
