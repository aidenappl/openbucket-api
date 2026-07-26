# Authentication

OpenBucket API is **self-contained for authentication**. It owns its own user table
(`users`), signs its own JWTs with its own key (`OB_JWT_SIGNING_KEY`), and issues its own
long-lived API tokens (`obk_…`).

An external identity provider can be plugged in as an **OIDC SSO source** — but it is only ever
an identity source. It never becomes the authority for a request. An API token belonging to the
SSO provider will **not** authenticate here; only credentials this service issued will.

---

## Credentials

Three credential types reach `middleware.AuthMiddleware`. All three resolve to a
`*structs.User` in the request context, retrievable with `middleware.GetUserFromContext`.

| Credential | Lifetime | Transport | Notes |
|-----------|----------|-----------|-------|
| **`obk_` API token** | `30d` / `90d` / `365d` / `never` | `Authorization: Bearer obk_…` | Stored only as a SHA-256 hash in `api_tokens`; resolved against the DB on every request, so **revocation is immediate** |
| **Access JWT** | 15 minutes | `Authorization: Bearer <jwt>` or `ob-access-token` cookie | HS-signed with `OB_JWT_SIGNING_KEY` |
| **Refresh JWT** | 7 days | `ob-refresh-token` cookie **only** | `/auth/refresh` reads no header — the cookie is the only accepted transport |

`validateToken` (`middleware/auth.go`) checks the `obk_` prefix **before** attempting to parse
the value as a JWT. An opaque API token is never run through the JWT parser.

Every path also re-checks `user.Active` — deactivating a user takes effect on their next
request, regardless of unexpired tokens.

---

## Endpoints

### Public

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/auth/login` | POST | Local email + password login |
| `/auth/refresh` | POST | Exchange the refresh cookie for a fresh token pair |
| `/auth/sso/config` | GET | Public SSO config — whether SSO is enabled, and its button label |
| `/auth/sso/login` | GET | Redirect to the SSO provider's authorization URL |
| `/auth/sso/callback` | GET | OAuth2 callback — provisions/links the user and sets cookies |

### Protected

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/auth/self` | GET | The authenticated user |
| `/auth/self` | PUT | Update own name / password |
| `/auth/logout` | POST | Clear auth cookies, drop the stored SSO session |

---

## Local login

```http
POST /auth/login
Content-Type: application/json

{ "email": "user@example.com", "password": "…" }
```

**The response body contains the user, not the tokens.** Tokens arrive as `Set-Cookie`:

| Cookie | HttpOnly | Purpose |
|--------|----------|---------|
| `ob-access-token` | yes | 15-minute access JWT |
| `ob-refresh-token` | yes | 7-day refresh JWT |
| `ob-logged-in` | **no** | Readable by JS purely so a frontend can detect login state |

All three are `SameSite=Lax`, scoped to `OB_COOKIE_DOMAIN`, and `Secure` unless
`OB_COOKIE_INSECURE=true`.

A non-browser client therefore has two options: scrape `Set-Cookie`, or — far better — use an
`obk_` API token.

> **⚠️ `/auth/login` only matches `auth_type = "local"`.**
> `query.GetUserByEmailAndAuthType(db.DB, email, "local")` is the lookup. An SSO-provisioned
> account cannot log in with a password no matter what it sends, and the failure is a generic
> `invalid credentials` — which reads like a wrong password but actually means *wrong auth
> path*. This is the single most confusing failure mode in this service.

---

## SSO

The SSO client (`sso/`) is a **generic OIDC/OAuth2 authorization-code client**. It is not
written against any one provider.

### Flow

1. Frontend calls `GET /auth/sso/config`. Response is `{"enabled": false}` when SSO is
   unconfigured, otherwise `{"enabled": true, "button_label": "…", "login_url": "/auth/sso/login"}`.
   Render the button only when `enabled` is true.
2. Browser navigates to `GET /auth/sso/login`. The server generates a random `state`, persists
   it in `settings` under `sso_state:<state>` with a 10-minute expiry, and 302s to the
   provider's authorize URL. If SSO is not configured this returns **404**, not 401.
3. Provider redirects back to `/auth/sso/callback?code=…&state=…`.
4. The callback validates `state`, exchanges the code, fetches userinfo, provisions or links the
   user, stores the provider's tokens, sets the same cookie trio as local login, and redirects
   to `OB_SSO_POST_LOGIN_URL`.

Failures redirect (they do not return JSON) with an error code in the query string:
`sso_denied`, `sso_missing_params`, `sso_state_expired`, `sso_exchange_failed`,
`sso_userinfo_failed`, `sso_no_email`, `sso_provision_failed`.

### Provider compatibility

`ExchangeCode` tries three token-endpoint conventions in order — JSON body, HTTP Basic auth
(RFC 6749 preferred), then credentials in the form body — and `doTokenRequest` accepts both a
flat OAuth2 token response and providers that wrap it in a `{"success": true, "data": {…}}`
envelope. `FetchUserInfo` unwraps the same envelope shape. This exists so that providers which
deviate from the spec still work; **do not remove the fallbacks when adding a new provider.**

### Auto-provisioning

With `OB_SSO_AUTO_PROVISION=true` (the default) an unrecognised SSO identity creates a user with
role `pending`. `RejectPending` then blocks them from `/core/v1/*` and `/admin/*` with a `403`
and error code `4004`, while `/auth/self` still works — so the new user can see their own status
while an admin approves them.

The user is matched on the field named by `OB_SSO_USER_IDENTIFIER` (default `email`), with the
provider's `sub` stored as `sso_subject`.

### Grant checkpointing

An SSO user's grant is re-validated against the provider's introspection endpoint on a
**5-minute TTL** (`ssoCheckpointTTL` in `middleware/auth.go`), using the stored refresh token.

- Provider reports `active: false` → the `sso_sessions` row is deleted and the request **401**s.
- **Network errors fail open.** A transient provider outage must not log everyone out. The
  tradeoff is a small extra revocation-latency budget during an incident — this is deliberate.

The provider's access and refresh tokens are encrypted at rest with `OB_CRYPTO_KEY`, the same
key used for stored S3 credentials.

---

## Refresh

```http
POST /auth/refresh
Cookie: ob-refresh-token=…
```

Reads the refresh token **from the cookie only** — there is no header form. Returns a fresh
token pair in `Set-Cookie` and the user in the body. Both tokens rotate.

## Logout

`POST /auth/logout` (protected) expires all three cookies. If the caller is an SSO user, the
`sso_sessions` row is deleted too, so the stored provider tokens do not outlive the session.

---

## CSRF

`CSRFMiddleware` implements the double-submit cookie pattern. A non-HttpOnly `ob-csrf` cookie is
issued on any request that lacks one; unsafe methods must echo it back in `X-CSRF-Token`.

**Skipped for:**

- Safe methods (`GET`, `HEAD`, `OPTIONS`)
- Any request with an `Authorization: Bearer …` header — stateless API clients are not subject
  to CSRF
- `/auth/login`, `/auth/refresh`, `/auth/sso/callback`

Failures return `403` with error code `4030` (missing cookie) or `4031` (mismatch).

> **Cookie-authenticated frontends must send `X-CSRF-Token` on every mutation.** This is the
> most common cause of an unexplained 403 in a newly built client.

---

## Roles

| Role | Access |
|------|--------|
| `admin` | Everything, including `/admin/*` |
| `editor` | Object and folder mutations |
| `viewer` | Read-only |
| `pending` | `/auth/self` only — awaiting admin approval |

`RequireAdmin` and `RequireEditor` wrap individual handlers; `RejectPending` is applied as
subrouter middleware on both `/core/v1/` and `/admin/`.

Distinguish the two failure codes: **401 `authentication required`** means no valid credential
was presented; **403 `admin access required`** means the credential was fine but the role was
not.

---

## API tokens

Long-lived opaque tokens for CLIs, CI and [openbucket-mcp](https://github.com/aidenappl/openbucket-mcp).

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

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/admin/api-tokens` | GET | List active tokens (values never returned) |
| `/admin/api-tokens` | POST | Create — plaintext returned exactly once |
| `/admin/api-tokens/{id}` | DELETE | Revoke (soft delete, audit trail kept) |

`expires_in` accepts `30d`, `90d`, `365d` or `never`; the default is `90d`.

---

## Configuration

All values are delivered via **Keyring** at startup (`env.Init()`), falling back to environment
variables for local development. Names only — never commit values.

| Variable | Default | Purpose |
|----------|---------|---------|
| `OB_JWT_SIGNING_KEY` | — (required) | Signs access and refresh JWTs |
| `OB_CRYPTO_KEY` | — (required) | AES-GCM key for stored S3 and SSO credentials |
| `OB_COOKIE_DOMAIN` | `` | Cookie domain, e.g. `.appleby.cloud` for cross-subdomain auth |
| `OB_COOKIE_INSECURE` | `false` | `true` drops the `Secure` flag for local HTTP dev |
| `OB_ADMIN_EMAIL` | `` | First-run bootstrap admin |
| `OB_ADMIN_PASSWORD` | `` | First-run bootstrap admin |
| `OB_SSO_CLIENT_ID` | `` | OIDC client ID |
| `OB_SSO_CLIENT_SECRET` | `` | OIDC client secret |
| `OB_SSO_AUTHORIZE_URL` | `` | Provider authorization endpoint |
| `OB_SSO_TOKEN_URL` | `` | Provider token endpoint |
| `OB_SSO_USERINFO_URL` | `` | Provider userinfo endpoint |
| `OB_SSO_INTROSPECT_URL` | `` | Provider introspection endpoint (grant checkpointing) |
| `OB_SSO_REDIRECT_URL` | `` | This service's `/auth/sso/callback` URL |
| `OB_SSO_LOGOUT_URL` | `` | Provider logout endpoint |
| `OB_SSO_SCOPES` | `openid email profile` | Requested scopes |
| `OB_SSO_USER_IDENTIFIER` | `email` | Userinfo field used to match/create the user |
| `OB_SSO_BUTTON_LABEL` | `Sign in with SSO` | Label returned by `/auth/sso/config` |
| `OB_SSO_AUTO_PROVISION` | `true` | Create a `pending` user for unknown SSO identities |
| `OB_SSO_POST_LOGIN_URL` | `` | Where the callback redirects on success |

SSO settings are **overridable at runtime from the database** — `sso.LoadConfig()` reads the
`sso.*` prefix from `settings` and only falls back to the environment when no rows exist. Admins
manage them through `GET|PUT /admin/sso-config`, so a provider can be swapped without a redeploy.

---

## See also

- [Frontend Integration](frontend-integration.md) — building a client against these endpoints
- [`AGENTS.md`](../AGENTS.md) — full repo guide
