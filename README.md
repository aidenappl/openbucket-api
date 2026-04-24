# openbucket-api

A Go-based API for S3-compatible bucket operations with Forta authentication.

## Requirements

- Go 1.25+
- MariaDB / MySQL database
- A registered platform in the Forta admin panel

## Environment Variables

### Required

| Variable                | Description                                                          |
| ----------------------- | -------------------------------------------------------------------- |
| `CORE_DB_DSN`           | MariaDB DSN (e.g., `user:pass@tcp(host:3306)/dbname?parseTime=true`) |
| `OB_CRYPTO_KEY`            | AES-GCM key for encrypting stored S3 credentials                     |
| `FORTA_CLIENT_ID`       | OAuth2 client ID from Forta admin panel                              |
| `FORTA_CLIENT_SECRET`   | OAuth2 client secret from Forta admin panel                          |
| `FORTA_CALLBACK_URL`    | Full callback URL (e.g., `https://myapp.example.com/forta/callback`) |
| `FORTA_JWT_SIGNING_KEY` | HMAC-SHA512 key for local Forta token validation                     |

### Optional

| Variable                      | Default | Description                                                |
| ----------------------------- | ------- | ---------------------------------------------------------- |
| `PORT`                        | `8000`  | Server port                                                |
| `FORTA_POST_LOGIN_REDIRECT`   | `/`     | Redirect after successful login                            |
| `FORTA_POST_LOGOUT_REDIRECT`  | `/`     | Redirect after logout                                      |
| `FORTA_COOKIE_DOMAIN`         | ``      | Cookie domain (e.g., `.appleby.cloud` for cross-subdomain) |
| `FORTA_COOKIE_INSECURE`       | `false` | Set to `true` for local HTTP development                   |
| `FORTA_FETCH_USER_ON_PROTECT` | `false` | Fetch full user profile on every protected request         |
| `FORTA_DISABLE_AUTO_REFRESH`  | `false` | Disable automatic token refresh                            |

## Database

Run the migration before starting the server:

```sql
source migrations/001_create_sessions.sql
```

## Forta Authentication Endpoints

| Endpoint        | Method | Description                           |
| --------------- | ------ | ------------------------------------- |
| `/forta/login`  | GET    | Redirects to Forta login page         |
| `/forta/logout` | GET    | Logs out and clears auth cookies      |
| `/self`         | GET    | Get current user info (requires auth) |

## Running

```bash
export CORE_DB_DSN="user:pass@tcp(localhost:3306)/openbucket?parseTime=true"
export OB_CRYPTO_KEY="your-32-byte-crypto-key"
export FORTA_CLIENT_ID="your-client-id"
export FORTA_CLIENT_SECRET="your-client-secret"
export FORTA_CALLBACK_URL="https://your-domain.com/forta/callback"
export FORTA_JWT_SIGNING_KEY="your-signing-key"

go run main.go
```

## Sessions

Sessions are stored in the database and associated with the authenticated Forta user. Create a session once via `POST /core/v1/session` — no client-side token or header is needed afterwards.

For bucket operation requests, include the numeric session ID (from `GET /core/v1/sessions`) in the URL path: `/core/v1/{sessionId}/...`. The server looks up the session by ID and verifies it belongs to the authenticated Forta user. Returns `400` for an invalid ID, `404` if not found, or `403` if the session belongs to another user.

Sessions are scoped to their owner — a user cannot access another user's session.

## Using Forta Protection in Handlers

All `/core/v1/` routes are wrapped with `forta.Protected`. To protect additional routes:

```go
import forta "github.com/aidenappl/go-forta"

r.HandleFunc("/protected", forta.Protected(myHandler)).Methods(http.MethodGet)

func myHandler(w http.ResponseWriter, r *http.Request) {
    fortaID, _ := forta.GetFortaIDFromContext(r.Context())
    // ...
}
```
