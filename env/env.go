package env

import (
	"fmt"
	"os"
)

var (
	Port    = getEnv("PORT", "8000")
	TLSCert = getEnv("TLS_CERT", "")
	TLSKey  = getEnv("TLS_KEY", "")

	CoreDBDSN = getEnvOrPanic("CORE_DB_DSN")
	CryptoKey = getEnvOrPanic("CRYPTO_KEY")

	// Forta Authentication Configuration
	FortaAppDomain          = getEnvOrPanic("FORTA_APP_DOMAIN")
	FortaAPIDomain          = getEnvOrPanic("FORTA_API_DOMAIN")
	FortaLoginDomain        = getEnvOrPanic("FORTA_LOGIN_DOMAIN")
	FortaClientID           = getEnvOrPanic("FORTA_CLIENT_ID")
	FortaClientSecret       = getEnvOrPanic("FORTA_CLIENT_SECRET")
	FortaCallbackURL        = getEnvOrPanic("FORTA_CALLBACK_URL")
	FortaJWTSigningKey      = getEnvOrPanic("FORTA_JWT_SIGNING_KEY")
	FortaPostLoginRedirect  = getEnv("FORTA_POST_LOGIN_REDIRECT", "/")
	FortaPostLogoutRedirect = getEnv("FORTA_POST_LOGOUT_REDIRECT", "/")
	FortaCookieDomain       = getEnv("FORTA_COOKIE_DOMAIN", "")
	FortaCookieInsecure     = getEnv("FORTA_COOKIE_INSECURE", "false") == "true"
	FortaFetchUserOnProtect = getEnv("FORTA_FETCH_USER_ON_PROTECT", "true") == "true"
	FortaDisableAutoRefresh = getEnv("FORTA_DISABLE_AUTO_REFRESH", "false") == "true"
)

func getEnv(key string, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}

	return fallback
}

func getEnvOrPanic(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		panic(fmt.Sprintf("❌ missing required environment variable: '%v'\n", key))
	}
	return value
}
