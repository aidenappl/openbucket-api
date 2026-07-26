# AGENTS.md — openbucket-api

> `openbucket-api` is the **control plane for OpenBucket**, the S3-compatible object storage
> platform. It owns its own user directory, brokers session-scoped access to buckets across
> multiple storage instances, and proxies admin operations to `openbucket-go`. This file
> orients any agent/worker before touching code in this repo.
>
> **⚠️ Golden rule — keep this file current:** any change that alters structure, stack,
> commands, conventions, endpoints, schema, or the auth/session contract MUST update this
> AGENTS.md in the SAME change. Stale context here misleads every future agent.
>
> **⚠️ Read this first:** OpenBucket has its **own user table and its own JWT signing key**. It
> supports a pluggable OIDC SSO provider as an identity source only — an API token issued by
> that provider does **not** authenticate here. This service issues `obk_` tokens of its own.

---

## What this repo is

A Go HTTP service (`module github.com/aidenappl/openbucket-api`) serving
**`api.openbucket.appleby.cloud`**. Three surfaces:

1. **Auth** (`/auth/*`) — local email+password login, SSO via the configured OIDC provider,
   refresh, self.
2. **Core v1** (`/core/v1/*`) — session creation, then every object and folder operation scoped
   to a session.
3. **Admin** (`/admin/*`) — users, storage instances, SSO config, API tokens, and a passthrough
   proxy to an instance's own admin API.

It owns access brokering, sessions and its user directory. It does **not** own the bytes —
actual storage lives in `openbucket-go` instances (`cdn.appleby.cloud`,
`cdn.trailblaze.to`).

## Stack & dependencies

- **Router:** `github.com/gorilla/mux`, with nested subrouters: `/core/v1/` → `/{sessionId}`.
- **SQL:** `Masterminds/squirrel` aliased `sq` over `database/sql` + MySQL driver. **No ORM.**
- **AWS SDK:** `github.com/aws/aws-sdk-go` — S3 operations against the instance endpoints.
- **JWT:** `github.com/golang-jwt/jwt/v5`, signed with `OB_JWT_SIGNING_KEY` (**distinct from
  any SSO provider's key**).
- **Secrets:** config is loaded from **Keyring** at startup via `env.Init()`.
- **CORS:** `github.com/rs/cors` with an explicit allowlist.

## Project structure

| Path | Package | Role |
|------|---------|------|
| `main.go` | `main` | `env.Init()` (Keyring) → `db.Init()` → `db.RunMigrations()` → `bootstrap.EnsureAdminUser()` → routes → optional TLS server. |
| `db/db.go` | `db` | Lazy `Init()` (not an IIFE — config comes from Keyring first), `Queryable`, `RunMigrations()`. |
| `db/migrations/` | — | Numbered `.sql` files, applied at startup and tracked in `migrations_applied`. |
| `env/env.go` | `env` | `Init()` resolves config from Keyring/env. |
| `bootstrap/` | `bootstrap` | `EnsureAdminUser` — first-run admin from `OB_ADMIN_EMAIL`/`OB_ADMIN_PASSWORD`. |
| `middleware/` | `middleware` | `AuthMiddleware`, `Protected`, `RequireAdmin`, `RequireEditor`, `RejectPending`, `SessionMiddleware`, `CSRFMiddleware`, logging. |
| `jwt/` | `jwt` | Access (15m) and refresh (7d) token minting/validation. Has tests. |
| `query/` | `query` | `users`, `sessions`, `settings`, `instances`, `sso_sessions`, `api_tokens`. |
| `routers/` | `routers` | Handlers. Some have tests (`HandleLogin_test.go`). |
| `structs/` | `structs` | `User`, `Session`, `Instance`, `SSOSession`, `ApiToken`. |
| `tools/` | `tools` | `Password`, `Crypto`, `Validate`, `DecodeSession`, `ApiToken`. Several have tests. |
| `sso/` | `sso` | Generic OIDC SSO client — no provider-specific code. |
| `responder/` | `responder` | Envelopes. **Note: no `QueryError` helper** — use `SendError(w, http.StatusInternalServerError, msg, err)`. |
| `aws/` | `aws` | S3 client wrappers and error mapping. |

## Running, building & testing

```bash
dev / dev build / dev test / dev fmt / dev vet / dev check / dev tidy
```

- **Port:** `env.Port`; deployed on `8004` (host 8003). Serves HTTPS directly when `TLS_CERT`
  and `TLS_KEY` are set, otherwise HTTP behind the proxy.
- **Health:** `GET /health` → bare `OK` (not a JSON envelope — MCP/HTTP clients must not assume
  JSON here).
- **Config (NAMES only):** `CORE_DB_DSN`, `OB_JWT_SIGNING_KEY`, `OB_CRYPTO_KEY`,
  `OB_COOKIE_DOMAIN`, `OB_COOKIE_INSECURE`, `OB_ADMIN_EMAIL`, `OB_ADMIN_PASSWORD`, `PORT`,
  `TLS_CERT`, `TLS_KEY`, plus the `OB_SSO_*` block. Delivered via Keyring
  (`KEYRING_ACCESS_KEY_ID` / `KEYRING_SECRET_ACCESS_KEY` / `KEYRING_URL`).

**Migrations run automatically** at startup and `log.Fatalf` on failure — so a healthy
container is proof its migrations applied. Nothing needs to be applied by hand.

## How code is written here

- **Handlers are `Handle{Verb}{Entity}.router.go`**, two-layer, `db.Queryable` first.
- **Use `responder.SendError(w, status, msg, err...)`** — this repo has no `QueryError`.
- **Session-scoped handlers read the session from context** via `middleware.GetSession`, and
  **the bucket always comes from the session**. Several handlers explicitly overwrite any
  caller-supplied bucket (`req.Bucket = session.BucketName` in `HandleModifyObjectACL` and
  `HandleCreateFolder`). Do not add a `bucket` parameter to a session-scoped endpoint; it will
  be silently ignored.
- **Path traversal is checked explicitly** — `strings.Contains(req.Key, "..")` rejects. Keep
  that check on any new key-accepting handler.
- **Query parameters are typed carefully.** `expiration` on presign is **integer seconds**
  (`strconv.Atoi`, default 3600, values over 86400 clamped to 3600) — not a duration string.

## Domain & architecture

### Auth — self-contained

| Credential | Lifetime | Notes |
|-----------|----------|-------|
| **`obk_` API token** | 30d/90d/365d/never | SHA-256 hash in `api_tokens`; resolved per request, so **revocation is immediate** |
| **Access JWT** | 15 minutes | `Authorization: Bearer` or `ob-access-token` cookie |
| **Refresh token** | 7 days | **Cookie only** (`ob-refresh-token`) — `/auth/refresh` reads no header |

`validateToken` checks the `obk_` prefix **before** attempting JWT parsing.

**`/auth/login` returns tokens in `Set-Cookie` headers, not the response body** — unlike the
common convention of returning a token pair in JSON. The body carries the user. Any non-browser
client must scrape `Set-Cookie` or use an `obk_` token.

**`/auth/login` looks users up with `auth_type = "local"`.** An SSO-provisioned account cannot
log in with a password no matter what it supplies — the failure is `invalid credentials`, which
looks like a wrong password but means "wrong auth path".

**SSO sessions are re-checkpointed** against the SSO provider on a 5-minute TTL
(`ssoCheckpointTTL`). If the IDP reports the grant inactive, the `sso_sessions` row is deleted
and the request 401s. Network errors **fail open** deliberately — a transient IDP outage must
not log everyone out.

**The SSO client is provider-agnostic.** `ExchangeCode` tries three token-endpoint conventions
(JSON body, HTTP Basic, form body) and `doTokenRequest`/`FetchUserInfo` accept both flat OAuth2
responses and providers that wrap them in a `{"success": true, "data": {…}}` envelope. Those
fallbacks exist because real providers deviate from the spec — do not remove them.

### CSRF

`CSRFMiddleware` implements double-submit cookie. It is **skipped entirely for
`Authorization: Bearer` requests**, since those are stateless API clients not subject to CSRF.
It is also skipped for safe methods and for `/auth/login`, `/auth/refresh`,
`/auth/sso/callback`. Cookie-authenticated mutations need `X-CSRF-Token` matching the `ob-csrf`
cookie.

### Sessions

A **session** binds a bucket, region, endpoint and credentials. Everything under
`/core/v1/{sessionId}/` operates within it. Sessions are the unit of access — there is no way to
address a bucket without one.

### Roles

`admin`, `editor`, `viewer`, `pending`. `RejectPending` blocks `pending` users from everything
except `/auth/self`, so a newly SSO-provisioned user can see their own status while awaiting
approval.

## Ecosystem & related repos

| Repo | Relationship |
|------|--------------|
| [`openbucket-go`](https://github.com/aidenappl/openbucket-go) | The storage server. Registered as **instances**; admin ops are proxied through `/admin/instances/{id}/proxy/{path}`. |
| [`openbucket-web`](https://github.com/aidenappl/openbucket-web) | Dashboard at `openbucket.appleby.cloud`. |
| [`openbucket-cli`](https://github.com/aidenappl/openbucket-cli) | CLI client. |
| [`openbucket-mcp`](https://github.com/aidenappl/openbucket-mcp) | MCP server. Uses `obk_` tokens. |
| `keyring-api` | Supplies configuration at startup. |

An external OIDC provider may be configured for SSO, but it is **not** an ecosystem dependency —
this service keeps its own users and no code or config names a specific provider.

## Operations

- **Deployed** as `registry.appleby.cloud/openbucket-api:latest` on Lattice stack 20
  (container id 192, host 8003 → container 8004), alongside `openbucket-web`.
- **CI:** `.github/workflows/push-to-registry.yml` builds, pushes, then triggers a Lattice
  deploy via `secrets.LATTICE_DEPLOY_URL?container=${{ secrets.IMAGE_NAME }}`.
  - **`IMAGE_NAME` is a secret and is deliberate** — image names do not always match the repo
    name. Do not "simplify" it to `github.event.repository.name`.
  - This step **silently failed from 2026-05-09 to 2026-07-23** with `HTTP 000` / curl exit 6
    (could not resolve host): the image built and pushed every time while nothing deployed.
    Root cause was a bad `LATTICE_DEPLOY_URL`, not this file. If a deploy seems not to land,
    check the deploy token's `last_used_at` in Lattice — a `null` there proves CI never
    reached the API.
- **Common failure modes:**
  - *`invalid credentials` on a correct password* — the account is `auth_type = "sso"`.
  - *403 `admin access required`* — authenticated but wrong role (distinct from 401
    `authentication required`).
  - *`Invalid expiration time` on presign* — a duration string was sent instead of seconds.
  - *Container healthy but routes 404* — an old image; the deploy step didn't run.

## Rules & guardrails

- **Never log, echo or print a token, password, crypto key or presigned URL.**
- **Never remove the `..` path-traversal checks** on key-accepting handlers.
- **Never add a caller-supplied `bucket` to a session-scoped endpoint** — the session is the
  authority and the handler overwrites it.
- **Never bypass `RejectPending`** on a new protected route.
- **Do not change the CSRF exemptions** without understanding that Bearer clients depend on the
  bypass; adding a blanket requirement breaks every API-token consumer.
- **Do not assume `/health` returns JSON.** It returns bare `OK`.
- **Migrations are forward-only and auto-applied.** Never edit a migration that has shipped —
  add a new numbered file.
- **Setting an object ACL to `public-read` exposes it to the internet.** Treat as a
  security-relevant change.

## Verification — always before "done"

```bash
gofmt -w .
go build ./...
go vet ./...
go test ./...     # jwt, middleware, responder, routers and tools packages have real tests
```

or `dev check`. For auth or route changes, verify against the deployed service:

```bash
curl -o /dev/null -w "%{http_code}\n" https://api.openbucket.appleby.cloud/health          # 200
curl -o /dev/null -w "%{http_code}\n" https://api.openbucket.appleby.cloud/admin/api-tokens # 401, not 404
```

A **404 where you expect 401 means the deployed image predates your change** — the route does
not exist yet. That distinction is the fastest way to tell "my code is wrong" from "my code
isn't deployed".

**Never report work complete if build, vet or tests fail.**

## Keeping this file updated

Update this AGENTS.md in the same change when you:
- **Add/rename/remove a route** → update the surface list and the auth table.
- **Add a migration** → note the schema change; migrations are auto-applied, so a stale
  description here is the only thing that can mislead.
- **Change auth, session, CSRF or role behaviour** → update *Domain & architecture*. These are
  the areas where a wrong assumption becomes a security bug.
- **Change a query-parameter type or unit** → say so explicitly; the presign seconds/duration
  mismatch shipped a broken MCP tool.
- **Change CI or the deploy step** → update *Operations*, including the `IMAGE_NAME` rationale.
- Also keep `README.md` and `docs/` in sync — `docs/authentication.md` is the detailed auth
  reference and `docs/frontend-integration.md` the client guide; both name concrete routes,
  cookies and env vars, so a route or config change invalidates them immediately.
