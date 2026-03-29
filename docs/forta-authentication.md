# Forta Authentication

OpenBucket API uses [Forta](https://forta.appleby.cloud) as its authentication provider via the `go-forta` library.

## Overview

Forta provides OAuth2-based authentication with automatic token refresh, cookie management, and both local and remote token validation strategies.

---

## Endpoints

### GET `/forta/login`

Initiates the OAuth2 login flow by redirecting the user to the Forta authorization server.

**Authentication:** None required

**Response:** `302 Redirect` to Forta login page

**Flow:**

1. Generates a random CSRF `state` value and stores it in a short-lived HttpOnly cookie
2. Redirects to `{FORTA_DOMAIN}/oauth/authorize` with:
   - `response_type=code`
   - `client_id`
   - `redirect_uri` (your callback URL)
   - `state` (CSRF token)

**Example:**

```
GET /forta/login
→ 302 Redirect to https://forta.appleby.cloud/oauth/authorize?...
```

---

### GET `/forta/callback`

OAuth2 callback handler that exchanges the authorization code for access tokens.

**Authentication:** None required

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|--------|----------|--------------------------------|
| `code` | string | Yes | Authorization code from Forta |
| `state` | string | Yes | CSRF state token |

**Response:**

- `302 Redirect` to `FORTA_POST_LOGIN_REDIRECT` on success
- `400 Bad Request` on missing parameters or CSRF mismatch

**Flow:**

1. Validates `state` against the CSRF cookie
2. Exchanges `code` for tokens via `POST {FORTA_DOMAIN}/auth/exchange`
3. Sets `forta-access-token` and `forta-refresh-token` HttpOnly cookies
4. Redirects to configured post-login URL

**Error Response:**

```json
{
  "error": "missing code or state parameter"
}
```

---

### GET `/forta/logout`

Logs out the user by clearing authentication cookies.

**Authentication:** None required

**Response:** `302 Redirect` to `FORTA_POST_LOGOUT_REDIRECT`

**Flow:**

1. Expires `forta-access-token` cookie
2. Expires `forta-refresh-token` cookie
3. Redirects to configured post-logout URL

**Example:**

```
GET /forta/logout
→ 302 Redirect to /
```

---

### GET `/forta/check`

Check if the current request has valid authentication without requiring it.

**Authentication:** None required (public endpoint)

**Response:** `200 OK`

**Response Body (authenticated):**

```json
{
  "authenticated": true,
  "user": {
    "id": 12345,
    "email": "user@example.com",
    "name": "John Doe",
    "display_name": "johnd"
  }
}
```

**Response Body (not authenticated):**

```json
{
  "authenticated": false,
  "message": "not authenticated"
}
```

**Use Cases:**

- Frontend apps checking login state on page load
- Conditionally showing login/logout buttons
- Personalizing public pages for logged-in users

---

### GET `/self`

Returns the currently authenticated user's profile information.

**Authentication:** Required (Forta session)

**Headers:**

```
Authorization: Bearer <access_token>
```

Or via `forta-access-token` cookie (set automatically after login)

**Response:** `200 OK`

**Response Body:**

```json
{
  "data": {
    "id": 12345,
    "email": "user@example.com",
    "name": "John Doe",
    "display_name": "johnd"
  },
  "message": "successfully retrieved current user"
}
```

**Error Response (401 Unauthorized):**

```json
{
  "error": "missing or invalid authorization"
}
```

**Notes:**

- The `email`, `name`, and `display_name` fields are only present when the server is configured with remote validation or `FORTA_FETCH_USER_ON_PROTECT=true`
- With local JWT validation only, just `id` is guaranteed

---

## Authentication Flow

### Browser-Based Login

```
┌─────────┐     ┌─────────────┐     ┌───────────────┐
│ Browser │     │ OpenBucket  │     │ Forta Server  │
└────┬────┘     └──────┬──────┘     └───────┬───────┘
     │                 │                    │
     │ GET /forta/login│                    │
     │────────────────>│                    │
     │                 │                    │
     │    302 Redirect │                    │
     │<────────────────│                    │
     │                 │                    │
     │ GET /oauth/authorize                 │
     │─────────────────────────────────────>│
     │                 │                    │
     │         User authenticates           │
     │<─────────────────────────────────────│
     │                 │                    │
     │ GET /forta/callback?code=...&state=..│
     │────────────────>│                    │
     │                 │                    │
     │                 │ POST /auth/exchange│
     │                 │───────────────────>│
     │                 │                    │
     │                 │   Access + Refresh │
     │                 │<───────────────────│
     │                 │                    │
     │ 302 + Set-Cookie│                    │
     │<────────────────│                    │
     │                 │                    │
```

### API Request with Token

```
┌─────────┐     ┌─────────────┐     ┌───────────────┐
│ Client  │     │ OpenBucket  │     │ Forta Server  │
└────┬────┘     └──────┬──────┘     └───────┬───────┘
     │                 │                    │
     │ GET /self       │                    │
     │ Cookie: forta-access-token=...       │
     │────────────────>│                    │
     │                 │                    │
     │                 │ Validate token     │
     │                 │ (local or remote)  │
     │                 │                    │
     │   200 OK        │                    │
     │   { user data } │                    │
     │<────────────────│                    │
     │                 │                    │
```

### Auto-Refresh Flow

When an access token expires and a valid refresh token exists:

```
┌─────────┐     ┌─────────────┐     ┌───────────────┐
│ Client  │     │ OpenBucket  │     │ Forta Server  │
└────┬────┘     └──────┬──────┘     └───────┬───────┘
     │                 │                    │
     │ GET /self       │                    │
     │ (expired token) │                    │
     │────────────────>│                    │
     │                 │                    │
     │                 │ POST /auth/refresh │
     │                 │───────────────────>│
     │                 │                    │
     │                 │   New tokens       │
     │                 │<───────────────────│
     │                 │                    │
     │ 200 OK          │                    │
     │ Set-Cookie: new tokens              │
     │<────────────────│                    │
     │                 │                    │
```

---

## Configuration

See environment variables in the main [README](../README.md#environment-variables).

| Variable                      | Description                              |
| ----------------------------- | ---------------------------------------- |
| `FORTA_DOMAIN`                | Forta server URL                         |
| `FORTA_CLIENT_ID`             | OAuth2 client ID                         |
| `FORTA_CLIENT_SECRET`         | OAuth2 client secret                     |
| `FORTA_CALLBACK_URL`          | Full callback URL for this service       |
| `FORTA_POST_LOGIN_REDIRECT`   | Where to redirect after login            |
| `FORTA_POST_LOGOUT_REDIRECT`  | Where to redirect after logout           |
| `FORTA_COOKIE_DOMAIN`         | Cookie domain for cross-subdomain auth   |
| `FORTA_JWT_SIGNING_KEY`       | Enable local token validation            |
| `FORTA_FETCH_USER_ON_PROTECT` | Fetch full profile with local validation |

---

## Error Responses

All error responses use this format:

```json
{
  "error": "human-readable message"
}
```

| Status                      | Scenario                                                 |
| --------------------------- | -------------------------------------------------------- |
| `400 Bad Request`           | Missing parameters, CSRF mismatch, or Forta server error |
| `401 Unauthorized`          | Missing/invalid/expired token (when auto-refresh fails)  |
| `500 Internal Server Error` | Forta not initialized                                    |
