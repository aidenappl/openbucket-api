package env

import (
	"context"
	"fmt"
	"net/url"
	"strings"

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
	SSOIssuerURL      string
	SSOTokenURL       string
	SSOUserInfoURL    string
	SSORedirectURL    string
	SSOLogoutURL      string
	SSOScopes         string
	SSOUserIdentifier string
	SSOButtonLabel    string
	SSOAutoProvision  bool
	SSOPostLoginURL   string

	// ActivityReportURL (optional): successful changes by SSO users are
	// mirrored here — the identity provider's per-user activity trail. Empty
	// reports nothing. See routers/activity_sink.go.
	ActivityReportURL string

	// StorageInternalEndpoints maps public storage endpoints to LAN ones,
	// "public=internal,…" — see aws.InitInternalEndpoints.
	StorageInternalEndpoints string

	// Monitor (zone appleby). Empty key: events go nowhere (stdout off).
	MonitorIngestURL string
	MonitorIngestKey string
)

// DefaultMonitorIngestURL is Monitor zone appleby's ingest endpoint.
const DefaultMonitorIngestURL = "https://appleby-monitor-api.appleby.cloud/v1/events"

// ingestURL returns the Monitor ingest endpoint for u: go-monitor needs the
// full events path, and a bare host (as the Keyring copy may hold) got 404s.
func ingestURL(u string) string {
	u = strings.TrimRight(strings.TrimSpace(u), "/")
	if u == "" {
		return ""
	}
	if parsed, err := url.Parse(u); err == nil && (parsed.Path == "" || parsed.Path == "/") {
		return u + "/v1/events"
	}
	return u
}

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
	// Set this to upgrade SSO from OAuth2 to OIDC — see sso.SSOConfig.IssuerURL.
	SSOIssuerURL = getOr(ctx, "OB_SSO_ISSUER_URL", "")
	SSOTokenURL = getOr(ctx, "OB_SSO_TOKEN_URL", "")
	SSOUserInfoURL = getOr(ctx, "OB_SSO_USERINFO_URL", "")
	SSORedirectURL = getOr(ctx, "OB_SSO_REDIRECT_URL", "")
	SSOLogoutURL = getOr(ctx, "OB_SSO_LOGOUT_URL", "")
	SSOScopes = getOr(ctx, "OB_SSO_SCOPES", "openid email profile")
	SSOUserIdentifier = getOr(ctx, "OB_SSO_USER_IDENTIFIER", "email")
	SSOButtonLabel = getOr(ctx, "OB_SSO_BUTTON_LABEL", "Sign in with SSO")
	SSOAutoProvision = getOr(ctx, "OB_SSO_AUTO_PROVISION", "true") == "true"
	SSOPostLoginURL = getOr(ctx, "OB_SSO_POST_LOGIN_URL", "")

	StorageInternalEndpoints = getOr(ctx, "OB_STORAGE_INTERNAL_ENDPOINTS", "")

	ActivityReportURL = strings.TrimSpace(getOr(ctx, "OB_ACTIVITY_REPORT_URL", ""))

	// Monitor, zone appleby. The documented ingest endpoint is the default;
	// OB_MONITOR_INGEST_URL overrides it. The shared Keyring
	// MONITOR_APPLEBY_INGEST_URL is not used: with it, every batch was
	// rejected (404, then 401 with both the shared and a dedicated key).
	MonitorIngestURL = ingestURL(getOr(ctx, "OB_MONITOR_INGEST_URL", DefaultMonitorIngestURL))
	// This service's own ingest key wins over the shared one. Trimmed: a
	// pasted value can carry a trailing newline.
	MonitorIngestKey = strings.TrimSpace(getOr(ctx, "OB_MONITOR_INGEST_KEY", getOr(ctx, "MONITOR_APPLEBY_INGEST_KEY", "")))

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
