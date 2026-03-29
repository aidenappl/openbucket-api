# OpenBucket API Documentation

## Table of Contents

- [Forta Authentication](forta-authentication.md) - OAuth2 authentication via Forta

## Quick Links

### Authentication Endpoints

| Endpoint          | Method | Auth | Description              |
| ----------------- | ------ | ---- | ------------------------ |
| `/forta/login`    | GET    | No   | Start OAuth2 login flow  |
| `/forta/callback` | GET    | No   | OAuth2 callback handler  |
| `/forta/logout`   | GET    | No   | Clear auth cookies       |
| `/forta/check`    | GET    | No   | Check auth status        |
| `/self`           | GET    | Yes  | Get current user profile |

### Core API Endpoints

| Endpoint            | Method | Description             |
| ------------------- | ------ | ----------------------- |
| `/`                 | GET    | API welcome message     |
| `/health`           | GET    | Health check            |
| `/core/v1/session`  | POST   | Create a bucket session |
| `/core/v1/sessions` | PUT    | Parse/validate sessions |

### Bucket Operations

All bucket operations require a valid session token via `Authorization: Bearer <token>`.

| Endpoint                           | Method | Description         |
| ---------------------------------- | ------ | ------------------- |
| `/core/v1/{bucket}/object`         | PUT    | Upload object       |
| `/core/v1/{bucket}/object`         | GET    | Get object          |
| `/core/v1/{bucket}/object`         | DELETE | Delete object       |
| `/core/v1/{bucket}/objects`        | GET    | List objects        |
| `/core/v1/{bucket}/object/head`    | GET    | Get object metadata |
| `/core/v1/{bucket}/object/acl`     | GET    | Get object ACL      |
| `/core/v1/{bucket}/object/acl`     | PUT    | Modify object ACL   |
| `/core/v1/{bucket}/object/presign` | GET    | Get presigned URL   |
| `/core/v1/{bucket}/object/rename`  | PUT    | Rename object       |
| `/core/v1/{bucket}/folder`         | GET    | Get folder          |
| `/core/v1/{bucket}/folder`         | POST   | Create folder       |
| `/core/v1/{bucket}/folder`         | PUT    | Update folder       |
| `/core/v1/{bucket}/folder`         | DELETE | Delete folder       |
| `/core/v1/{bucket}/folders`        | GET    | List folders        |
