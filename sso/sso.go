package sso

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	ssolib "github.com/aidenappl/go-forta/sso"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/env"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/tools"
)

// SSOConfig holds all SSO configuration values.
type SSOConfig struct {
	Enabled        bool
	ClientID       string
	ClientSecret   string
	AuthorizeURL   string
	TokenURL       string
	UserInfoURL    string
	IntrospectURL  string
	RedirectURL    string
	LogoutURL      string
	Scopes         string
	UserIdentifier string
	ButtonLabel    string
	AutoProvision  bool
	PostLoginURL   string
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// LoadConfig reads SSO configuration from the database.
// Falls back to environment variables if DB settings don't exist.
func LoadConfig() *SSOConfig {
	settings, err := query.GetSettingsByPrefix(db.DB, "sso.")
	if err != nil || len(settings) == 0 {
		return &SSOConfig{
			Enabled:        env.SSOClientID != "" && env.SSOAuthorizeURL != "",
			ClientID:       env.SSOClientID,
			ClientSecret:   env.SSOClientSecret,
			AuthorizeURL:   env.SSOAuthorizeURL,
			TokenURL:       env.SSOTokenURL,
			UserInfoURL:    env.SSOUserInfoURL,
			IntrospectURL:  env.SSOIntrospectURL,
			RedirectURL:    env.SSORedirectURL,
			LogoutURL:      env.SSOLogoutURL,
			Scopes:         env.SSOScopes,
			UserIdentifier: env.SSOUserIdentifier,
			ButtonLabel:    env.SSOButtonLabel,
			AutoProvision:  env.SSOAutoProvision,
			PostLoginURL:   env.SSOPostLoginURL,
		}
	}

	cfg := &SSOConfig{
		Enabled:        settings["sso.enabled"] == "true",
		ClientID:       strings.TrimSpace(settings["sso.client_id"]),
		AuthorizeURL:   strings.TrimSpace(settings["sso.authorize_url"]),
		TokenURL:       strings.TrimSpace(settings["sso.token_url"]),
		UserInfoURL:    strings.TrimSpace(settings["sso.userinfo_url"]),
		IntrospectURL:  strings.TrimSpace(or(settings["sso.introspect_url"], env.SSOIntrospectURL)),
		RedirectURL:    strings.TrimSpace(settings["sso.redirect_url"]),
		LogoutURL:      strings.TrimSpace(settings["sso.logout_url"]),
		Scopes:         strings.TrimSpace(or(settings["sso.scopes"], "openid email profile")),
		UserIdentifier: strings.TrimSpace(or(settings["sso.user_identifier"], "email")),
		ButtonLabel:    or(settings["sso.button_label"], "Sign in with SSO"),
		AutoProvision:  settings["sso.auto_provision"] != "false",
		PostLoginURL:   strings.TrimSpace(or(settings["sso.post_login_url"], env.SSOPostLoginURL)),
	}

	// Decrypt client secret from DB
	if secret, ok := settings["sso.client_secret"]; ok && secret != "" {
		decrypted, err := tools.Decrypt(secret)
		if err == nil {
			cfg.ClientSecret = decrypted
		} else {
			log.Printf("WARNING: failed to decrypt SSO client secret, SSO may not work: %v", err)
		}
	}

	return cfg
}

// PostLoginRedirectURL returns the URL to redirect users to after SSO login.
func (c *SSOConfig) PostLoginRedirectURL() string {
	if c.PostLoginURL != "" && c.PostLoginURL != "/" {
		return c.PostLoginURL
	}
	if c.RedirectURL != "" {
		if u, err := url.Parse(c.RedirectURL); err == nil {
			u.Path = "/"
			u.RawQuery = ""
			return u.String()
		}
	}
	return "/"
}

func IsConfigured() bool {
	cfg := LoadConfig()
	return cfg.Enabled && cfg.ClientID != "" && cfg.AuthorizeURL != "" && cfg.TokenURL != ""
}

// Config returns the public SSO configuration for the frontend login page.
func Config() map[string]any {
	cfg := LoadConfig()
	if !cfg.Enabled || cfg.ClientID == "" || cfg.AuthorizeURL == "" || cfg.TokenURL == "" {
		return map[string]any{"enabled": false}
	}
	return map[string]any{
		"enabled":      true,
		"button_label": cfg.ButtonLabel,
		"login_url":    "/auth/sso/login",
	}
}

// StateCookie binds an in-flight login to the browser that started it.
//
// ⚠️ THIS DID NOT EXIST BEFORE. Without it, state was validated only against the
// database, so any party who could induce a callback with a state value they had
// observed could complete a login in a victim's browser. The cookie means a
// callback arriving in a different browser than the one that started the login is
// refused before any database work happens.
const StateCookie = "openbucket-sso-state"

const statePrefix = "sso_state:"

// StateStore implements ssolib.StateStore over the settings table.
type StateStore struct{}

// NewStateStore returns a StateStore over the package-level DB handle.
func NewStateStore() *StateStore { return &StateStore{} }

// SaveState persists an in-flight login record and best-effort sweeps dead ones.
func (s *StateStore) SaveState(_ context.Context, state string, data []byte, _ time.Time) error {
	if err := query.SetSetting(db.DB, statePrefix+state, string(data)); err != nil {
		return fmt.Errorf("sso: persist state: %w", err)
	}
	go sweepExpiredStates()
	return nil
}

// ConsumeState atomically returns and deletes a state record. See
// query.DeleteSettingExisted for why the DELETE, not the read, decides the winner.
func (s *StateStore) ConsumeState(_ context.Context, state string) ([]byte, error) {
	key := statePrefix + state

	raw, err := query.GetSetting(db.DB, key)
	if err != nil || raw == "" {
		return nil, ssolib.ErrNoState
	}

	deleted, err := query.DeleteSettingExisted(db.DB, key)
	if err != nil {
		return nil, fmt.Errorf("sso: consume state: %w", err)
	}
	if !deleted {
		return nil, ssolib.ErrNoState
	}
	return []byte(raw), nil
}

// sweepExpiredStates prunes expired or unparseable records. Unparseable includes
// every record written in the OLD format, which stored a bare RFC3339 timestamp
// rather than JSON — those can never be consumed, so deleting them is correct.
func sweepExpiredStates() {
	states, err := query.GetSettingsByPrefix(db.DB, statePrefix)
	if err != nil {
		return
	}
	for k, v := range states {
		var sd ssolib.StateData
		if err := json.Unmarshal([]byte(v), &sd); err != nil {
			_ = query.DeleteSetting(db.DB, k)
			continue
		}
		if time.Now().After(sd.ExpiresAt) {
			_ = query.DeleteSetting(db.DB, k)
		}
	}
}

// LoginHandler redirects the user to the provider's authorization URL.
//
// Now via the library, so the URL carries a PKCE S256 challenge — this service sent
// none before. The verifier and nonce live SERVER-SIDE in the state record, so the
// callback validates against values the browser never held.
//
// The crypto/rand failure that used to PANIC here now returns an error: a library
// that panics takes the process down, and a failed login is the right blast radius
// for a randomness failure this rare.
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	cfg := LoadConfig()
	if !cfg.Enabled || cfg.ClientID == "" || cfg.AuthorizeURL == "" {
		http.Error(w, "SSO not configured", http.StatusNotFound)
		return
	}

	provider := cfg.Provider()
	adapter, err := ssolib.NewAdapter(r.Context(), provider)
	if err != nil {
		log.Printf("sso: adapter build failed: %v", err)
		http.Error(w, "SSO misconfigured", http.StatusInternalServerError)
		return
	}

	state, nonce, verifier, err := ssolib.GenerateState(r.Context(), NewStateStore(), provider.Slug, "")
	if err != nil {
		log.Printf("sso: state generation failed: %v", err)
		http.Error(w, "failed to initialize login", http.StatusInternalServerError)
		return
	}

	authURL, err := adapter.AuthCodeURL(state, nonce, verifier)
	if err != nil {
		log.Printf("sso: authorize url build failed: %v", err)
		http.Error(w, "failed to initialize login", http.StatusInternalServerError)
		return
	}

	// SameSite=Lax so the cookie survives the top-level GET redirect back from the
	// IdP but is not sent on cross-site subresource requests. Path scoped to
	// /auth/sso so it is presented on nothing else.
	http.SetCookie(w, &http.Cookie{
		Name:     StateCookie,
		Value:    state,
		Path:     "/auth/sso",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   !env.CookieInsecure,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, authURL, http.StatusFound)
}

// ClearStateCookie expires the browser-side binding after a callback.
func ClearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     StateCookie,
		Value:    "",
		Path:     "/auth/sso",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   !env.CookieInsecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// WHAT USED TO BE HERE, AND WHY IT IS GONE
//
// ExchangeCode, exchangeWithJSON / exchangeWithBasicAuth / exchangeWithBodyAuth,
// doTokenRequest, FetchUserInfo, GetUserIdentifier, GetUserName, GetUserPicture,
// GetUserEmail and a local Introspect all lived here. They are now in
// github.com/aidenappl/go-forta/sso, shared with monitor-core and lattice-api.
//
// The defects that made sharing necessary, so nobody restores them:
//
//  1. THE THREE-SHAPE TOKEN EXCHANGE tried a JSON body, then HTTP Basic, then body
//     credentials, in sequence. Against an authorization server that treats codes
//     as single-use — which forta-api provably does — the first attempt to reach
//     the server consumes the code, so the fallbacks could only ever get
//     invalid_grant. A guaranteed wasted round trip on every login.
//  2. NO PKCE. No code_challenge, no verifier: a leaked authorization code was
//     redeemable by whoever held it.
//  3. GetUserIdentifier READ sso.user_identifier AND FELL BACK TO EMAIL, and the
//     result was stored as the subject. Identity keyed on a reassignable address is
//     an account-takeover primitive.
//
// Do not re-add a local copy of any of it.
// ─────────────────────────────────────────────────────────────────────────────
