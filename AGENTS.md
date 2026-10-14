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
- **SSO:** `github.com/aidenappl/go-oidc-session` — the shared, provider-agnostic OIDC client
  (BFF pattern, RFC 10017). It runs the login, holds the provider's tokens server-side in
  `oidc_sessions`, refreshes them when due and ends the session when the provider refuses.
  Pinned at **v0.1.0**; upgrade with `go get github.com/aidenappl/go-oidc-session@vX.Y.Z`.

## Project structure

| Path | Package | Role |
|------|---------|------|
| `main.go` | `main` | `env.Init()` (Keyring) → `db.Init()` → `db.RunMigrations()` → `bootstrap.EnsureAdminUser()` → routes → optional TLS server. |
| `db/db.go` | `db` | Lazy `Init()` (not an IIFE — config comes from Keyring first), `Queryable`, `RunMigrations()`. |
| `db/migrations/` | — | Numbered `.sql` files, applied at startup and tracked in `migrations_applied`. |
| `env/env.go` | `env` | `Init()` resolves config from Keyring/env. |
| `bootstrap/` | `bootstrap` | `EnsureAdminUser` — first-run admin from `OB_ADMIN_EMAIL`/`OB_ADMIN_PASSWORD`. |
| `middleware/` | `middleware` | `AuthMiddleware`, `Protected`, `RequireAdmin`, `RequireEditor`, `RejectPending`, `SessionMiddleware`, `CSRFMiddleware`, logging. `correlation.go`: inbound request/trace ids, forwarded on SSO token refreshes. |
| `jwt/` | `jwt` | Access (15m) and refresh (7d) token minting/validation. Has tests. |
| `query/` | `query` | `users`, `sessions`, `settings`, `instances`, `api_tokens`. SSO sessions/states are `go-oidc-session/sqlstore`. |
| `routers/` | `routers` | Handlers. Some have tests (`HandleLogin_test.go`). |
| `structs/` | `structs` | `User`, `Session`, `Instance`, `ApiToken`. |
| `tools/` | `tools` | `Password`, `Crypto`, `Validate`, `DecodeSession`, `ApiToken`. Several have tests. |
| `sso/` | `sso` | SSO config (`sso.*` settings) and `session.go`: the `go-oidc-session` `Manager` (rebuilt by `Reload()` on config save), the user resolver, and the token `Sealer` over `tools.Encrypt`. No provider-specific code. |
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
  (`presignExpiration`: default 3600; ≤ 0 or non-numeric → 400; values over 86400 are clamped to
  **86400**) — not a duration string. The response's `expiration` is what was actually granted;
  clients must display that, not what they asked for. Until October 2026 over-max values were
  silently reset to 3600, so a "7-day" link from openbucket-web lasted one hour.

## Domain & architecture

### Auth — self-contained

| Credential | Lifetime | Notes |
|-----------|----------|-------|
| **`obk_` API token** | 30d/90d/365d/never | SHA-256 hash in `api_tokens`; resolved per request, so **revocation is immediate** |
| **Access JWT** | 15 minutes | `Authorization: Bearer` or `ob-access-token` cookie |
| **Refresh token** | 7 days | **Cookie only** (`ob-refresh-token`) — `/auth/refresh` reads no header. **Local-password users only.** |
| **SSO session** | the identity provider's | `openbucket_session` cookie (opaque, HttpOnly, host-only on the API). SSO users get **no** OpenBucket JWTs. |

`validateToken` checks the `obk_` prefix **before** attempting JWT parsing.

**`/auth/login` returns tokens in `Set-Cookie` headers, not the response body** — unlike the
common convention of returning a token pair in JSON. The body carries the user. Any non-browser
client must scrape `Set-Cookie` or use an `obk_` token.

**`/auth/login` looks users up with `auth_type = "local"`.** An SSO-provisioned account cannot
log in with a password no matter what it supplies — the failure is `invalid credentials`, which
looks like a wrong password but means "wrong auth path".

### SSO sessions — lifetime is the identity provider's

SSO users are authenticated by `sso.Manager().Authenticate` on the `openbucket_session` cookie
(`middleware/auth.go`). The cookie's SHA-256 keys an `oidc_sessions` row (migration 011) holding
the provider's access/refresh/id tokens, encrypted with `tools.Encrypt`.

| Event | Result |
|---|---|
| provider access token within 1 min of expiry | refreshed server-side on the next request (single-flight per session) |
| refresh succeeds | session continues — a successful refresh *is* the liveness check |
| provider answers `invalid_grant` | session deleted → 401; the frontend re-authenticates silently |
| provider unreachable | serve until access-token expiry + 10 min, then **503** (never 401) |
| back-channel logout | matching sessions deleted at once |

There is no introspection checkpoint and no separate OpenBucket clock for SSO users. Their
OpenBucket JWTs are refused by `validateToken` and their refresh cookies by `/auth/refresh`
(`auth_type = "sso"`), so pre-migration sessions re-authenticate once and land on the new model.

**Silent re-authentication:** `GET /auth/sso/login?prompt=none&return_to=<url>` (the frontend's
`/login` does this automatically). A user still signed in at the provider comes straight back;
otherwise the callback lands on `/login?error=login_required`. `return_to` is accepted only on the
post-login URL's origin.

**Logout** (`POST /auth/logout`) ends **only OpenBucket's** session (go-oidc-session ≥ v0.1.2 is
local by default) and returns `data.redirect_url` — `/login?signed_out=1` when an SSO session was
present, `""` otherwise. Forta and other apps stay signed in. The frontend navigates to
it. Register `<frontend>/login?signed_out=1` as a **logout redirect URI** with the provider.

**Correlation.** `AuthMiddleware` builds a ctx with `correlatedContext(r)` (inbound `X-Request-ID`,
trace id from `traceparent`/`X-Trace-ID`, each kept only if a UUID or 8–64 hex) and passes it to
`Authenticate`, so a token refresh is findable in the provider's logs by the same id.

**The SSO protocol lives in the shared module now.** This file previously said `ExchangeCode`
tried three token-endpoint conventions (JSON body, HTTP Basic, form body) and instructed **"do
not remove them"**. That instruction was wrong, and following it kept a real defect alive:

- Against an authorization server that treats codes as **single-use** — which any conforming one
  does, and forta-api provably does — the first attempt to reach the server **consumes the
  code**, so the fallbacks could only ever receive `invalid_grant`. It was never a compatibility
  layer; it was a guaranteed wasted round trip on every login, visible in forta-api's logs as a
  400 immediately followed by a 200.
- There was **no PKCE at all**, so a leaked authorization code was redeemable by anyone holding
  it. The library now sends an S256 challenge on every authorization request.
- `GetUserIdentifier` returned whatever `sso.user_identifier` named — **email** by default — and
  the result was stored as the subject. Identity keyed on a reassignable address is an
  account-takeover primitive. The library reads the standard `sub`; `user_identifier` is retained
  in the admin API for compatibility and **read by nothing**.
- `ValidateState` read then unconditionally deleted, so two concurrent callbacks presenting the
  same state **both passed**. `query.DeleteSettingExisted` makes the DELETE decide the winner.
- There was **no browser binding** on state. The library's `openbucket_session_login` cookie adds it.
- `crypto/rand` failure **panicked**, taking the process down. It now fails the login.

The `{"success": true, "data": {…}}` envelope tolerance is the one fallback that survived, and it
lives in the library's OAuth2 adapter — several first-party APIs here wrap every response that
way.

### ⚠️ `sso.issuer_url` — OAuth2 vs OIDC is a security choice, not a preference

The adapter is chosen by whether an issuer is configured (`SSOConfig.kind()`):

| `sso.issuer_url` | Adapter | What you get |
|---|---|---|
| set | `KindOIDC` | discovery, a **signed id_token**, nonce verification, and `sid` |
| empty | `KindOAuth2` | no id_token — identity comes from an **unsigned** UserInfo call |

Without an issuer, anything able to obtain an access token can become that user, and there is no
`sid`, so session-scoped back-channel logout is impossible. The OAuth2 fallback exists only so an
existing deployment survives the upgrade; it is a state to leave, not to stay in. forta-api has
published a conforming discovery document since Phase 1 — set `https://auth.appleby.cloud`.

⚠️ **`sid` is captured only at login and cannot be backfilled** (migration 010 adds the column).
Sessions established before the issuer is set have `sid NULL` forever and are reachable only by
subject-wide logout. Users must re-login once after the switch.

### Back-channel logout (`POST /auth/sso/backchannel-logout`)

Handled by `sso.Manager().BackchannelHandler`: it verifies the logout token against the
provider's JWKS and deletes the `oidc_sessions` rows by `sid` (else `sub`). Because SSO users
hold no OpenBucket JWTs, deleting the row ends their access at the next request.

⚠️ **It is exempt from `CSRFMiddleware` and must stay exempt.** The notification is a
server-to-server POST with no cookie and no auth header, so it otherwise hits the double-submit
check it can never satisfy and is refused 403 (4030) — the provider retries six times, marks the
delivery exhausted, and revocation silently stays at poll speed. monitor-core shipped that on
2026-08-08 with a *passing* routing test, because the router was never what rejected it.
`middleware/csrf_test.go` covers this half; a routing test cannot.

⚠️ **It requires `sso.issuer_url`.** Verification needs the provider's JWKS, which only the OIDC
adapter discovers; with an OAuth2 provider the handler answers **501** rather than acting on a
token it cannot verify.

### CSRF

`CSRFMiddleware` implements double-submit cookie. It is **skipped for `Authorization: Bearer`
requests that carry no session cookie** (`ob-access-token` or `openbucket_session`) — stateless
API clients are not subject to CSRF, but a bearer header must not exempt a browser that also has a
session, or a forged request could add a junk header and ride it.
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

- **CI runners:** every GitHub Actions job runs on `[self-hosted, lattice-worker]` — a
  single-use runner started by `aidenappl/gh-runner-broker` on the CI host (`aplb-actions`,
  10.15.10.78), with the toolchain and shared Go/npm caches provisioned by
  `TeamTrailblaze/runner-setup`. Images build through the host's `lattice-builder` BuildKit
  (`builder: lattice-builder`), never `docker/setup-buildx-action`, which races other runners
  on the shared Docker daemon; its local layer cache replaces the gha cache. `setup-go` runs
  with `cache: false`. Jobs stuck in "Queued" mean the broker or the host is down — check the
  `gh-runner-broker` service in Monitor (zone `appleby`).

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
    reached the API. A bad URL like that (curl exit 6) is now retried 5 times (~75s) before the job fails.
  - **Network retries:** the appleby.cloud origin is on-prem behind Cloudflare on one WAN and ~5% of requests time out or 522, so the deploy workflow retries every call to it: registry login is a shell `docker login --password-stdin` loop (not `docker/login-action`, which cannot retry); the image is built with `load: true, provenance: false` and pushed by a separate `docker push` loop (`shell: bash`, so pipefail catches a failed push behind `tee`), so a blip retries the push (uploaded layers are skipped), not the build; and the Lattice deploy `curl` is retried. Each loop makes 5 attempts (5/10/20/40s), emits a `::warning::` per retry, retries curl failures and 408/429/502/503/504/520–524, and fails at once on auth errors / any other 4xx and on a Lattice 500 (the response body is now printed on failure). A 524 retry can redeploy the same commit twice — harmless. The pushed tag is a plain single-platform manifest (no provenance attestation).
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

**`sso_sessions` is dropped** (migration 012): the pre-go-oidc-session table, unused since `oidc_sessions` replaced it.
