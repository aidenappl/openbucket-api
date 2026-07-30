package routers

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	ssolib "github.com/aidenappl/go-forta/sso"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/sso"
	"github.com/aidenappl/openbucket-api/structs"
)

// HandleSSOConfig returns the public SSO configuration for the frontend login page.
func HandleSSOConfig(w http.ResponseWriter, r *http.Request) {
	cfg := sso.Config()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"success":true,"data":%s}`, mustJSON(cfg))
}

// HandleSSOLogin redirects the user to the SSO provider's authorization URL.
func HandleSSOLogin(w http.ResponseWriter, r *http.Request) {
	sso.LoginHandler(w, r)
}

// HandleSSOCallback handles the OAuth2 callback from the SSO provider.
func HandleSSOCallback(w http.ResponseWriter, r *http.Request) {
	cfg := sso.LoadConfig()

	// Check for errors from provider
	if errCode := r.URL.Query().Get("error"); errCode != "" {
		redirectWithError(w, r, cfg, "sso_denied")
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		redirectWithError(w, r, cfg, "sso_missing_params")
		return
	}

	// The browser-bound cookie is checked FIRST — it is the cheapest gate and it
	// refuses a callback arriving in a browser that did not start this login, before
	// any database work happens. It did not exist before this change.
	stateCookie, cookieErr := r.Cookie(sso.StateCookie)
	sso.ClearStateCookie(w)
	if cookieErr != nil || stateCookie.Value == "" ||
		subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(state)) != 1 {
		log.Printf("SSO: state cookie missing or does not match callback state")
		redirectWithError(w, r, cfg, "sso_state_expired")
		return
	}

	// The server-side record carries the PKCE verifier and nonce, and consuming it is
	// ATOMIC — see sso.StateStore.ConsumeState. The old ValidateState read then
	// deleted unconditionally and returned a bool, so two concurrent callbacks
	// presenting the same state both passed.
	stateData, err := ssolib.ConsumeState(r.Context(), sso.NewStateStore(), state)
	if err != nil {
		log.Printf("SSO: invalid, expired or already-consumed state")
		redirectWithError(w, r, cfg, "sso_state_expired")
		return
	}

	provider := cfg.Provider()
	adapter, err := ssolib.NewAdapter(r.Context(), provider)
	if err != nil {
		log.Printf("SSO: adapter build failed: %v", err)
		redirectWithError(w, r, cfg, "sso_exchange_failed")
		return
	}

	// ── One exchange, with the PKCE verifier attached ────────────────────────
	//
	// Replaces ExchangeCode's JSON → Basic → body fallback chain, which could never
	// have worked against single-use codes: the first attempt to reach the server
	// consumed the code, so the fallbacks could only ever get invalid_grant. It also
	// dropped the verifier entirely.
	identity, tokens, err := adapter.Exchange(r.Context(), code, stateData.Verifier, stateData.Nonce)
	if err != nil {
		log.Printf("SSO token exchange failed: %v", err)
		redirectWithError(w, r, cfg, "sso_exchange_failed")
		return
	}

	email := identity.Email
	if email == "" {
		redirectWithError(w, r, cfg, "sso_no_email")
		return
	}

	// ── The subject is the REAL `sub` ────────────────────────────────────────
	//
	// From the library, which reads the standard claim. The old code read `sub` then
	// `id` from an unvalidated UserInfo map; more importantly sso.GetUserIdentifier
	// existed alongside it and returned an email, and the two disagreed about what
	// identity meant.
	subject := identity.Subject

	name := ""
	if identity.Name != nil {
		name = *identity.Name
	}
	picture := ""
	if identity.Picture != nil {
		picture = *identity.Picture
	}

	// Resolve or create user
	user := resolveOrCreateSSOUser(cfg, email, subject, name, picture)
	if user == nil {
		redirectWithError(w, r, cfg, "sso_provision_failed")
		return
	}

	if !user.Active {
		redirectWithError(w, r, cfg, "sso_account_disabled")
		return
	}

	// Update profile image on each login
	if picture != "" {
		_, _ = query.UpdateUser(db.DB, user.ID, query.UpdateUserRequest{
			ProfileImageURL: &picture,
		})
	}

	// Persist the IdP tokens for the checkpoint. Encryption at rest is the
	// SessionStore's job — see sso/sessionstore.go.
	//
	// Non-fatal now, where it used to abort the login: the authorization has already
	// succeeded and the user should get their session. The cost of a failure is that
	// this session is not checkpointed until the next login, which is strictly better
	// than refusing a valid login over a cache write.
	if err := sso.NewSessionStore().SaveSession(r.Context(), int64(user.ID), ssolib.Session{
		Provider: sso.ProviderSlug,
		Subject:  subject,
		Tokens:   *tokens,
	}); err != nil {
		log.Printf("SSO: failed to persist sso session: %v", err)
	}
	if !setAuthCookies(w, user.ID) {
		return
	}
	http.Redirect(w, r, cfg.PostLoginRedirectURL(), http.StatusFound)
}

func resolveOrCreateSSOUser(cfg *sso.SSOConfig, email, subject, name, picture string) *structs.User {
	// 1. Try by SSO subject
	if subject != "" {
		if user, err := query.GetUserBySSOSubject(db.DB, subject); err == nil && user != nil {
			return user
		}
	}

	// 2. Try by email + auth_type=sso, healing the stored subject.
	if user, err := query.GetUserByEmailAndAuthType(db.DB, email, "sso"); err == nil && user != nil {
		// ── Heal the stored subject ──────────────────────────────────────────
		//
		// Three cases, all writing the real `sub` over what is there: nil, empty, or
		// AN EMAIL ADDRESS. The third is the repair: sso.GetUserIdentifier used to
		// return whatever `sso.user_identifier` named — "email" by default — and the
		// result was stored as the subject. A real `sub` from forta-api is a UUID and
		// can never contain "@", so sso.LooksLikeEmail cannot misfire on a legitimate
		// value.
		//
		// Healing on login rather than by migration is deliberate: the correct `sub`
		// is only knowable from a live token, so a script would have had to match on
		// email and trust that mapping.
		//
		// ⚠️ This user was matched BY EMAIL to get here, which is the very thing being
		// moved away from. It is safe only because the match is scoped to
		// auth_type='sso' and happens once — after this write the subject lookup
		// succeeds and this path is never taken for them again. When every row has a
		// real subject, delete the email fallback.
		if subject != "" && (user.SSOSubject == nil || *user.SSOSubject == "" || sso.LooksLikeEmail(*user.SSOSubject)) {
			previous := "nil"
			if user.SSOSubject != nil {
				previous = *user.SSOSubject
			}
			if err := query.UpdateUserSSOSubject(db.DB, user.ID, subject); err != nil {
				log.Printf("SSO: failed to heal sso_subject for user %d: %v", user.ID, err)
			} else {
				log.Printf("SSO: healed sso_subject for user %d from %q to the real OIDC sub", user.ID, previous)
			}
		}
		return user
	}

	// 3. Auto-provision
	if !cfg.AutoProvision {
		return nil
	}

	var ssoSubject, namePtr, picturePtr *string
	if subject != "" {
		ssoSubject = &subject
	}
	if name != "" {
		namePtr = &name
	}
	if picture != "" {
		picturePtr = &picture
	}

	user, err := query.CreateUser(db.DB, query.CreateUserRequest{
		Email:           email,
		Name:            namePtr,
		AuthType:        "sso",
		SSOSubject:      ssoSubject,
		ProfileImageURL: picturePtr,
		Role:            "pending",
	})
	if err != nil {
		log.Printf("SSO auto-provision failed: %v", err)
		return nil
	}
	return user
}

func redirectWithError(w http.ResponseWriter, r *http.Request, cfg *sso.SSOConfig, errorCode string) {
	redirectURL := cfg.PostLoginRedirectURL()
	u, err := url.Parse(redirectURL)
	if err != nil {
		http.Error(w, "SSO configuration error", http.StatusInternalServerError)
		return
	}
	q := u.Query()
	q.Set("error", errorCode)
	u.RawQuery = q.Encode()
	u.Path = "/login"
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func mustJSON(v map[string]any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
