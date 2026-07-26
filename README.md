# openbucket-api

A Go-based API for S3-compatible bucket operations, with its own user directory and pluggable
OIDC SSO.

## Requirements

- Go 1.25+
- MariaDB / MySQL database
- Keyring access for configuration (or the equivalent environment variables for local dev)

## Environment Variables

Configuration is resolved from **Keyring** at startup via `env.Init()`, falling back to
environment variables.

### Required

| Variable              | Description                                                          |
| --------------------- | -------------------------------------------------------------------- |
| `CORE_DB_DSN`         | MariaDB DSN (e.g., `user:pass@tcp(host:3306)/dbname?parseTime=true`) |
| `OB_CRYPTO_KEY`       | AES-GCM key for encrypting stored S3 and SSO credentials             |
| `OB_JWT_SIGNING_KEY`  | Signing key for this service's access and refresh JWTs               |

### Optional

| Variable             | Default | Description                                                |
| -------------------- | ------- | ---------------------------------------------------------- |
| `PORT`               | `8000`  | Server port                                                |
| `TLS_CERT`           | ``      | Serve HTTPS directly when set alongside `TLS_KEY`          |
| `TLS_KEY`            | ``      | Private key for `TLS_CERT`                                 |
| `OB_COOKIE_DOMAIN`   | ``      | Cookie domain (e.g., `.appleby.cloud` for cross-subdomain) |
| `OB_COOKIE_INSECURE` | `false` | Set to `true` for local HTTP development                   |
| `OB_ADMIN_EMAIL`     | ``      | First-run bootstrap admin                                  |
| `OB_ADMIN_PASSWORD`  | ``      | First-run bootstrap admin                                  |

### SSO (optional)

Leave unset to run with local accounts only. These are also editable at runtime through
`GET|PUT /admin/sso-config`, which takes precedence over the environment.

| Variable                 | Default                | Description                            |
| ------------------------ | ---------------------- | -------------------------------------- |
| `OB_SSO_CLIENT_ID`       | ``                     | OIDC client ID                         |
| `OB_SSO_CLIENT_SECRET`   | ``                     | OIDC client secret                     |
| `OB_SSO_AUTHORIZE_URL`   | ``                     | Provider authorization endpoint        |
| `OB_SSO_TOKEN_URL`       | ``                     | Provider token endpoint                |
| `OB_SSO_USERINFO_URL`    | ``                     | Provider userinfo endpoint             |
| `OB_SSO_INTROSPECT_URL`  | ``                     | Provider introspection endpoint        |
| `OB_SSO_REDIRECT_URL`    | ``                     | This service's `/auth/sso/callback` URL |
| `OB_SSO_LOGOUT_URL`      | ``                     | Provider logout endpoint               |
| `OB_SSO_SCOPES`          | `openid email profile` | Requested scopes                       |
| `OB_SSO_USER_IDENTIFIER` | `email`                | Userinfo field used to match the user  |
| `OB_SSO_BUTTON_LABEL`    | `Sign in with SSO`     | Label returned by `/auth/sso/config`   |
| `OB_SSO_AUTO_PROVISION`  | `true`                 | Create a `pending` user for unknown identities |
| `OB_SSO_POST_LOGIN_URL`  | ``                     | Redirect target after a successful login |

## Database

Migrations in `db/migrations/` are **applied automatically at startup** by `db.RunMigrations()`
and tracked in `migrations_applied`. Startup fails loudly if a migration fails, so a healthy
process is proof its migrations ran. Nothing needs to be run by hand.

Migrations are forward-only — never edit one that has shipped; add a new numbered file.

## API Tokens

`/admin/*` and `/core/v1/*` accept a long-lived opaque API token in the `Authorization` header,
as an alternative to the 15-minute access JWT. These are what CLIs, CI and
[openbucket-mcp](https://github.com/aidenappl/openbucket-mcp) use.

```bash
# Mint one (requires an admin session)
curl -X POST https://api.openbucket.appleby.cloud/admin/api-tokens \
  -H "Authorization: Bearer <access-token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"claude-code","expires_in":"365d"}'

# Use it
curl -H "Authorization: Bearer obk_..." \
  https://api.openbucket.appleby.cloud/admin/instances
```

| Endpoint                 | Method | Description                                     |
| ------------------------ | ------ | ----------------------------------------------- |
| `/admin/api-tokens`      | GET    | List active tokens (values never returned)      |
| `/admin/api-tokens`      | POST   | Create a token — plaintext returned exactly once |
| `/admin/api-tokens/{id}` | DELETE | Revoke a token (soft delete, audit trail kept)  |

Tokens are `obk_`-prefixed, stored only as a SHA-256 hash, and resolved against the database on
each request — so revocation is immediate. `expires_in` accepts `30d`, `90d`, `365d`, or
`never` (default `90d`).

OpenBucket has its own user directory and JWT signing key; an SSO provider is an identity source
only. An API token issued by that provider will **not** authenticate here.

Bearer auth also exempts a request from the CSRF double-submit check, which only applies to
cookie-authenticated calls.

## Authentication Endpoints

| Endpoint             | Method | Auth | Description                            |
| -------------------- | ------ | ---- | -------------------------------------- |
| `/auth/login`        | POST   | No   | Local email + password login           |
| `/auth/refresh`      | POST   | No   | Rotate tokens using the refresh cookie |
| `/auth/sso/config`   | GET    | No   | Public SSO config for the login page   |
| `/auth/sso/login`    | GET    | No   | Redirect to the SSO provider           |
| `/auth/sso/callback` | GET    | No   | OAuth2 callback (server-side)          |
| `/auth/self`         | GET    | Yes  | Get current user info                  |
| `/auth/self`         | PUT    | Yes  | Update own name / password             |
| `/auth/logout`       | POST   | Yes  | Log out and clear auth cookies         |

Tokens are set as `ob-access-token` / `ob-refresh-token` cookies, **not** returned in the
response body. See [docs/authentication.md](docs/authentication.md) for the full model.

## Running

```bash
export CORE_DB_DSN="user:pass@tcp(localhost:3306)/openbucket?parseTime=true"
export OB_CRYPTO_KEY="your-32-byte-crypto-key"
export OB_JWT_SIGNING_KEY="your-signing-key"

go run main.go
```

## Sessions

Sessions are stored in the database and associated with the authenticated user. Create a session
once via `POST /core/v1/session` — no client-side token or header is needed afterwards.

For bucket operation requests, include the numeric session ID (from `GET /core/v1/sessions`) in
the URL path: `/core/v1/{sessionId}/...`. The server looks up the session by ID and verifies it
belongs to the authenticated user. Returns `400` for an invalid ID, `404` if not found, or `403`
if the session belongs to another user.

Sessions are scoped to their owner — a user cannot access another user's session.

## Protecting Handlers

Auth lives in `middleware/`. Wrap an individual handler with `middleware.Protected`, or apply
`middleware.AuthMiddleware` to a subrouter:

```go
import "github.com/aidenappl/openbucket-api/middleware"

r.HandleFunc("/protected", middleware.Protected(myHandler)).Methods(http.MethodGet)

// Role-gated:
admin.HandleFunc("/things", middleware.RequireAdmin(myHandler)).Methods(http.MethodGet)

func myHandler(w http.ResponseWriter, r *http.Request) {
    user, ok := middleware.GetUserFromContext(r.Context())
    if !ok || user == nil {
        responder.SendError(w, http.StatusUnauthorized, "authentication required")
        return
    }
    // ...
}
```

`middleware.RejectPending` is applied as subrouter middleware on `/core/v1/` and `/admin/` —
keep it on any new protected subrouter.

## Documentation

- [Authentication](docs/authentication.md) — credentials, login flows, SSO, CSRF, roles
- [Frontend Integration](docs/frontend-integration.md) — building a browser client
- [`AGENTS.md`](AGENTS.md) — full contributor/agent guide
