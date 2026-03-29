# openbucket-api

A Go-based API for S3-compatible bucket operations with Forta authentication.

## Requirements

- Go 1.22+
- A registered platform in the Forta admin panel

## Environment Variables

### Required

| Variable              | Description                                                          |
| --------------------- | -------------------------------------------------------------------- |
| `JWT_SECRET`          | Secret key for signing session JWTs                                  |
| `CRYPTO_KEY`          | Key for encrypting S3 credentials                                    |
| `FORTA_CLIENT_ID`     | OAuth2 client ID from Forta admin panel                              |
| `FORTA_CLIENT_SECRET` | OAuth2 client secret from Forta admin panel                          |
| `FORTA_CALLBACK_URL`  | Full callback URL (e.g., `https://myapp.example.com/forta/callback`) |

### Optional

| Variable                      | Default                       | Description                                                |
| ----------------------------- | ----------------------------- | ---------------------------------------------------------- |
| `PORT`                        | `8000`                        | Server port                                                |
| `FORTA_DOMAIN`                | `https://forta.appleby.cloud` | Forta authentication server URL                            |
| `FORTA_POST_LOGIN_REDIRECT`   | `/`                           | Redirect after successful login                            |
| `FORTA_POST_LOGOUT_REDIRECT`  | `/`                           | Redirect after logout                                      |
| `FORTA_COOKIE_DOMAIN`         | ``                            | Cookie domain (e.g., `.appleby.cloud` for cross-subdomain) |
| `FORTA_COOKIE_INSECURE`       | `false`                       | Set to `true` for local HTTP development                   |
| `FORTA_JWT_SIGNING_KEY`       | ``                            | HMAC-SHA512 key for local token validation                 |
| `FORTA_FETCH_USER_ON_PROTECT` | `false`                       | Fetch full user profile even with local validation         |
| `FORTA_DISABLE_AUTO_REFRESH`  | `false`                       | Disable automatic token refresh                            |

## Forta Authentication Endpoints

| Endpoint          | Method | Description                           |
| ----------------- | ------ | ------------------------------------- |
| `/forta/login`    | GET    | Redirects to Forta login page         |
| `/forta/callback` | GET    | OAuth2 callback handler               |
| `/forta/logout`   | GET    | Logs out and clears auth cookies      |
| `/forta/check`    | GET    | Check authentication status (public)  |
| `/self`           | GET    | Get current user info (requires auth) |

## Running

```bash
# Set required environment variables
export JWT_SECRET="your-jwt-secret"
export CRYPTO_KEY="your-crypto-key"
export FORTA_CLIENT_ID="your-client-id"
export FORTA_CLIENT_SECRET="your-client-secret"
export FORTA_CALLBACK_URL="https://your-domain.com/forta/callback"

# Run the server
go run main.go
```

## Using Forta Protection in Handlers

To protect a route with Forta authentication:

```go
import forta "github.com/aidenappl/go-forta"

// Wrap your handler with forta.Protected
r.HandleFunc("/protected", forta.Protected(myHandler)).Methods(http.MethodGet)

func myHandler(w http.ResponseWriter, r *http.Request) {
    // Get user ID (always available in Protected handlers)
    fortaID, _ := forta.GetFortaIDFromContext(r.Context())

    // Get full user profile (when available)
    user, hasUser := forta.GetUserFromContext(r.Context())
    if hasUser {
        fmt.Fprintf(w, "Hello, %s", user.Email)
    }
}
```
