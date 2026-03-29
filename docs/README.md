# OpenBucket API Documentation

## Table of Contents

- [Forta Authentication](forta-authentication.md) - OAuth2 authentication via Forta
- [Frontend Integration](frontend-integration.md) - How to integrate Forta auth in your frontend

## Quick Links

### Authentication Endpoints

| Endpoint        | Method | Auth | Description              |
| --------------- | ------ | ---- | ------------------------ |
| `/forta/login`  | GET    | No   | Start OAuth2 login flow  |
| `/forta/logout` | GET    | No   | Clear auth cookies       |
| `/self`         | GET    | Yes  | Get current user profile |

### Core API Endpoints

All `/core/v1/` endpoints require Forta authentication (cookie-based). Bucket operation endpoints resolve the session automatically from the authenticated user + bucket name.

| Endpoint            | Method | Description                        |
| ------------------- | ------ | ---------------------------------- |
| `/`                 | GET    | API welcome message                |
| `/health`           | GET    | Health check                       |
| `/core/v1/session`  | POST   | Create a bucket session            |
| `/core/v1/sessions` | GET    | List all sessions for current user |

### Bucket Operations

All bucket operations require Forta authentication (cookie). Pass the numeric session ID (obtained from `GET /core/v1/sessions`) in the URL path — the API resolves the bucket and S3 credentials from that ID and verifies ownership against the authenticated Forta user.

Returns `400` for an invalid ID, `404` if not found, or `403` if the session belongs to a different user.

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
