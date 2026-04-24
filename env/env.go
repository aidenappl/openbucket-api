package env

import (
	"context"
	"fmt"

	keyring "github.com/aidenappl/go-keyring"
)

var (
	Port    string
	TLSCert string
	TLSKey  string

	CoreDBBase string
	CryptoKey  string

	// JWT Configuration
	JWTSigningKey string

	// Cookie Configuration
	CookieDomain   string
	CookieInsecure bool

	// Bootstrap Admin
	AdminEmail    string
	AdminPassword string

	// SSO Configuration (env var fallback — DB settings take precedence)
	SSOClientID       string
	SSOClientSecret   string
	SSOAuthorizeURL   string
	SSOTokenURL       string
	SSOUserInfoURL    string
	SSORedirectURL    string
	SSOLogoutURL      string
	SSOScopes         string
	SSOUserIdentifier string
	SSOButtonLabel    string
	SSOAutoProvision  bool
	SSOPostLoginURL   string
)

// Init loads all configuration from Keyring. The three KEYRING_* env vars
// (KEYRING_URL, KEYRING_ACCESS_KEY_ID, KEYRING_SECRET_ACCESS_KEY) must be
// set in the process environment. All application secrets are fetched from
// the Keyring API. Call this once at the top of main() before any other
// initialisation.
func Init() {
	fmt.Println("Loading configuration from Keyring...")
	ctx := context.Background()

	Port = getOr(ctx, "PORT", "8000")
	TLSCert = getOr(ctx, "TLS_CERT", "")
	TLSKey = getOr(ctx, "TLS_KEY", "")

	CoreDBBase = keyring.MustGet("CORE_DB_DSN")
	CryptoKey = keyring.MustGet("OB_CRYPTO_KEY")

	// JWT — required for local auth
	JWTSigningKey = keyring.MustGet("OB_JWT_SIGNING_KEY")

	// Cookies
	CookieDomain = getOr(ctx, "OB_COOKIE_DOMAIN", "")
	CookieInsecure = getOr(ctx, "OB_COOKIE_INSECURE", "false") == "true"

	// Bootstrap admin (optional — only used on first run)
	AdminEmail = getOr(ctx, "OB_ADMIN_EMAIL", "")
	AdminPassword = getOr(ctx, "OB_ADMIN_PASSWORD", "")

	// SSO (optional — DB settings take precedence)
	SSOClientID = getOr(ctx, "OB_SSO_CLIENT_ID", "")
	SSOClientSecret = getOr(ctx, "OB_SSO_CLIENT_SECRET", "")
	SSOAuthorizeURL = getOr(ctx, "OB_SSO_AUTHORIZE_URL", "")
	SSOTokenURL = getOr(ctx, "OB_SSO_TOKEN_URL", "")
	SSOUserInfoURL = getOr(ctx, "OB_SSO_USERINFO_URL", "")
	SSORedirectURL = getOr(ctx, "OB_SSO_REDIRECT_URL", "")
	SSOLogoutURL = getOr(ctx, "OB_SSO_LOGOUT_URL", "")
	SSOScopes = getOr(ctx, "OB_SSO_SCOPES", "openid email profile")
	SSOUserIdentifier = getOr(ctx, "OB_SSO_USER_IDENTIFIER", "email")
	SSOButtonLabel = getOr(ctx, "OB_SSO_BUTTON_LABEL", "Sign in with SSO")
	SSOAutoProvision = getOr(ctx, "OB_SSO_AUTO_PROVISION", "true") == "true"
	SSOPostLoginURL = getOr(ctx, "OB_SSO_POST_LOGIN_URL", "")

	fmt.Println("Connecting to Keyring... ✅ Done")
}

// getOr returns the keyring value for key, or fallback if the key is absent.
func getOr(ctx context.Context, key, fallback string) string {
	v, err := keyring.Get(ctx, key)
	if err != nil {
		return fallback
	}
	return v
}
