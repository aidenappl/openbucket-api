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

	// Forta Authentication Configuration
	FortaAppDomain          string
	FortaAPIDomain          string
	FortaLoginDomain        string
	FortaClientID           string
	FortaClientSecret       string
	FortaCallbackURL        string
	FortaJWTSigningKey      string
	FortaPostLoginRedirect  string
	FortaPostLogoutRedirect string
	FortaCookieDomain       string
	FortaCookieInsecure     bool
	FortaFetchUserOnProtect bool
	FortaDisableAutoRefresh bool
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
	CryptoKey = keyring.MustGet("CRYPTO_KEY")

	FortaAppDomain = keyring.MustGet("OB_FORTA_APP_DOMAIN")
	FortaAPIDomain = keyring.MustGet("FORTA_API_DOMAIN")
	FortaLoginDomain = keyring.MustGet("FORTA_LOGIN_DOMAIN")
	FortaClientID = keyring.MustGet("OB_FORTA_CLIENT_ID")
	FortaClientSecret = keyring.MustGet("OB_FORTA_CLIENT_SECRET")
	FortaCallbackURL = keyring.MustGet("OB_FORTA_CALLBACK_URL")
	FortaJWTSigningKey = keyring.MustGet("FORTA_JWT_SIGNING_KEY")
	FortaPostLoginRedirect = getOr(ctx, "FORTA_POST_LOGIN_REDIRECT", "/")
	FortaPostLogoutRedirect = getOr(ctx, "FORTA_POST_LOGOUT_REDIRECT", "/")
	FortaCookieDomain = getOr(ctx, "FORTA_COOKIE_DOMAIN", "")
	FortaCookieInsecure = getOr(ctx, "FORTA_COOKIE_INSECURE", "false") == "true"
	FortaFetchUserOnProtect = getOr(ctx, "FORTA_FETCH_USER_ON_PROTECT", "true") == "true"
	FortaDisableAutoRefresh = getOr(ctx, "FORTA_DISABLE_AUTO_REFRESH", "false") == "true"
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
