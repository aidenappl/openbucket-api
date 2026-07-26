# OpenBucket API Documentation

## Table of Contents

- [Authentication](authentication.md) - Credentials, local login, OIDC SSO, CSRF and roles
- [Frontend Integration](frontend-integration.md) - Building a browser client against this API

## Quick Links

### Authentication Endpoints

| Endpoint             | Method | Auth | Description                          |
| -------------------- | ------ | ---- | ------------------------------------ |
| `/auth/login`        | POST   | No   | Local email + password login         |
| `/auth/refresh`      | POST   | No   | Rotate tokens using the refresh cookie |
| `/auth/sso/config`   | GET    | No   | Public SSO config for the login page |
| `/auth/sso/login`    | GET    | No   | Redirect to the SSO provider         |
| `/auth/sso/callback` | GET    | No   | OAuth2 callback (server-side)        |
| `/auth/self`         | GET    | Yes  | Get current user profile             |
| `/auth/self`         | PUT    | Yes  | Update own name / password           |
| `/auth/logout`       | POST   | Yes  | Clear auth cookies                   |

Tokens are returned in `Set-Cookie`, not the response body. Cookie-authenticated mutations
require an `X-CSRF-Token` header matching the `ob-csrf` cookie; `Authorization: Bearer` clients
are exempt.

### Core API Endpoints

All `/core/v1/` endpoints require authentication — an access JWT (header or `ob-access-token`
cookie) or an `obk_` API token. A session must be created before any bucket operation.

| Endpoint            | Method | Description                        |
| ------------------- | ------ | ---------------------------------- |
| `/`                 | GET    | API welcome message                |
| `/health`           | GET    | Health check (bare `OK`, not JSON) |
| `/core/v1/session`  | POST   | Create a bucket session            |
| `/core/v1/session/{id}` | DELETE | Delete a session               |
| `/core/v1/sessions` | GET    | List all sessions for current user |

### Bucket Operations

Pass the numeric session ID (from `GET /core/v1/sessions`) in the URL path — the API resolves
the bucket and S3 credentials from that ID and verifies ownership against the authenticated
user.

Returns `400` for an invalid ID, `404` if not found, or `403` if the session belongs to a
different user.

| Endpoint                              | Method | Description         |
| ------------------------------------- | ------ | ------------------- |
| `/core/v1/{sessionId}/object`         | PUT    | Upload object       |
| `/core/v1/{sessionId}/object`         | GET    | Get object          |
| `/core/v1/{sessionId}/object`         | DELETE | Delete object       |
| `/core/v1/{sessionId}/objects`        | GET    | List objects        |
| `/core/v1/{sessionId}/object/head`    | GET    | Get object metadata |
| `/core/v1/{sessionId}/object/head`    | POST   | Get metadata (bulk) |
| `/core/v1/{sessionId}/object/acl`     | GET    | Get object ACL      |
| `/core/v1/{sessionId}/object/acl`     | PUT    | Modify object ACL   |
| `/core/v1/{sessionId}/object/presign` | GET    | Get presigned URL   |
| `/core/v1/{sessionId}/object/rename`  | PUT    | Rename object       |
| `/core/v1/{sessionId}/folder`         | GET    | Get folder          |
| `/core/v1/{sessionId}/folder`         | POST   | Create folder       |
| `/core/v1/{sessionId}/folder`         | PUT    | Update folder       |
| `/core/v1/{sessionId}/folder`         | DELETE | Delete folder       |
| `/core/v1/{sessionId}/folders`        | GET    | List folders        |

### Admin Endpoints

All require the `admin` role.

| Endpoint                    | Method              | Description                       |
| --------------------------- | ------------------- | --------------------------------- |
| `/admin/users`              | GET, POST           | User management                   |
| `/admin/users/{id}`         | PUT, DELETE         | Update / delete a user            |
| `/admin/api-tokens`         | GET, POST           | `obk_` token management           |
| `/admin/api-tokens/{id}`    | DELETE              | Revoke a token                    |
| `/admin/sso-config`         | GET, PUT            | Runtime SSO configuration         |
| `/admin/instances`          | GET, POST           | Storage instance registry         |
| `/admin/instances/{id}`     | PUT, DELETE         | Update / delete an instance       |
| `/admin/instances/{id}/proxy/{path}` | GET, POST, PUT, DELETE | Passthrough to an instance's admin API |
