package routers

import (
	"context"
	"log"
	"net/http"

	ssolib "github.com/aidenappl/go-forta/sso"
	"github.com/aidenappl/openbucket-api/sso"
)

// HandleBackchannelLogout is the OIDC Back-Channel Logout 1.0 receiver
// (POST /auth/sso/backchannel-logout).
//
// ─────────────────────────────────────────────────────────────────────────────
// WHAT IT BUYS, AND WHY THE CHECKPOINT REMAINS THE GUARANTEE
//
// The introspection checkpoint re-checks the upstream grant every five minutes,
// so without this a revoked grant keeps working for up to five minutes. This
// endpoint closes that window: the provider POSTs a signed logout_token the
// instant a grant is revoked and the session ends on arrival.
//
// It does NOT replace the checkpoint. Back-channel logout is best-effort BY
// SPECIFICATION — notifications are lost, endpoints go down, retries exhaust — so
// the poll stays the guarantee and this is the fast path. Do not relax
// CheckpointInterval because this exists.
//
// ⚠️ REQUIRES sso.issuer_url TO BE SET. Verification needs the provider's JWKS,
// which only the OIDC adapter discovers; with an OAuth2 provider go-forta answers
// 501 and says so, rather than acting on a token it cannot verify. That refusal
// is correct — accepting an unverifiable POST here would let anyone who can reach
// this URL log out any user by guessing a subject.
//
// ⚠️ UNAUTHENTICATED IN THE ORDINARY SENSE: no cookie, no bearer token. Its
// authentication IS the signature on the logout token. It is therefore exempt
// from CSRFMiddleware — see the note there — and must never be moved behind
// session middleware.
// ─────────────────────────────────────────────────────────────────────────────
var backchannelLogout = &ssolib.BackchannelLogout{
	// The SAME session store the checkpoint uses. That is what makes it a
	// BackchannelLogoutTarget, so push and poll end sessions through one code
	// path — including stamping tokens_revoked_at, without which deleting the row
	// would leave OpenBucket's own JWTs working.
	Sessions: sso.NewSessionStore(),
	Providers: func(_ context.Context, _ string) (*ssolib.Provider, error) {
		// Re-read on every call rather than caching: a provider whose client
		// secret or issuer changes in the admin UI is re-resolved instead of
		// pinned to whatever was loaded at boot.
		return sso.LoadConfig().Provider(), nil
	},
	Logf: log.Printf,
}

// HandleBackchannelLogout serves the receiver.
func HandleBackchannelLogout(w http.ResponseWriter, r *http.Request) {
	backchannelLogout.Handler(sso.ProviderSlug).ServeHTTP(w, r)
}
