# Frontend Integration Guide

How to build a browser client against the OpenBucket API. For the underlying auth model — token
types, cookie names, role behaviour — see [Authentication](authentication.md).

The reference implementation is [`openbucket-web`](https://github.com/aidenappl/openbucket-web);
its `src/services/*.service.ts` files are the canonical version of everything below.

---

## Overview

Auth is **cookie-based**. Two login paths reach the same place:

- **Local** — `POST /auth/login` with email + password.
- **SSO** — navigate to `GET /auth/sso/login`, which 302s to the configured OIDC provider.

Both set the same three cookies on success:

| Cookie | HttpOnly | Purpose |
|--------|----------|---------|
| `ob-access-token` | yes | 15-minute access JWT |
| `ob-refresh-token` | yes | 7-day refresh JWT |
| `ob-logged-in` | **no** | Readable by JS so the app can detect login state without a round trip |

Every request must send `credentials: "include"`.

**There is no public callback or check endpoint to poll.** Auth state comes from
`GET /auth/self`: `200` means authenticated, `401` means not.

> **Access tokens are not refreshed automatically.** The API does not silently renew an expired
> access token — on `401` the client must `POST /auth/refresh` and retry. See
> [Handling 401](#handling-401) below.

---

## Quick start

### 1. Decide which login options to show

`GET /auth/sso/config` is public and tells you whether the SSO button should exist at all.

```tsx
interface SSOConfig {
  enabled: boolean;
  button_label?: string;
  login_url?: string; // "/auth/sso/login"
}

async function getSSOConfig(): Promise<SSOConfig> {
  const res = await fetch(`${API_URL}/auth/sso/config`, {
    credentials: "include",
  });
  const data = await res.json();
  return data.data;
}
```

Render the SSO button only when `enabled` is true — an unconfigured provider returns
`{"enabled": false}`, and hitting `/auth/sso/login` anyway yields a `404`.

### 2. Local login

```tsx
async function login(email: string, password: string): Promise<User> {
  const res = await fetch(`${API_URL}/auth/login`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });

  if (!res.ok) throw new Error("invalid credentials");
  const data = await res.json();
  return data.data; // the user — tokens arrive as Set-Cookie, not in the body
}
```

> An SSO-provisioned account **cannot** log in here. `/auth/login` only matches
> `auth_type = "local"`, and the failure is a generic `invalid credentials` — which looks like a
> wrong password but means the user should have used the SSO button.

### 3. SSO login button

```tsx
function SSOLoginButton({ config }: { config: SSOConfig }) {
  if (!config.enabled) return null;

  const handleLogin = () => {
    sessionStorage.setItem("returnUrl", window.location.pathname);
    window.location.href = `${API_URL}/auth/sso/login`;
  };

  return <button onClick={handleLogin}>{config.button_label}</button>;
}
```

### 4. Logout

Logout is a **protected POST**, not a navigation — so it needs the CSRF header.

```tsx
async function logout(): Promise<void> {
  await fetch(`${API_URL}/auth/logout`, {
    method: "POST",
    credentials: "include",
    headers: { "X-CSRF-Token": getCookie("ob-csrf") ?? "" },
  });
  window.location.href = "/login";
}
```

### 5. Get the current user

```tsx
interface User {
  id: number;
  email: string;
  name: string | null;
  auth_type: "local" | "sso";
  profile_image_url: string | null;
  role: "admin" | "editor" | "viewer" | "pending";
  active: boolean;
  inserted_at: string;
  updated_at: string;
}

async function getCurrentUser(): Promise<User | null> {
  const res = await fetch(`${API_URL}/auth/self`, { credentials: "include" });
  if (res.status === 401) return null;
  if (!res.ok) throw new Error("failed to fetch user");
  const data = await res.json();
  return data.data;
}
```

---

## CSRF

The API issues a non-HttpOnly `ob-csrf` cookie. **Every cookie-authenticated mutation must echo
it back in an `X-CSRF-Token` header.** This is the most common cause of an unexplained `403` in
a newly built client.

```tsx
function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp(`(^| )${name}=([^;]+)`));
  return match ? decodeURIComponent(match[2]) : null;
}

function csrfHeaders(): Record<string, string> {
  const token = getCookie("ob-csrf");
  return token ? { "X-CSRF-Token": token } : {};
}
```

Exempt from the check: safe methods (`GET`/`HEAD`/`OPTIONS`), any request using
`Authorization: Bearer`, and `/auth/login`, `/auth/refresh`, `/auth/sso/callback`.

Failures return `403` with error code `4030` (missing cookie) or `4031` (mismatch).

---

## Sessions

A **session** binds a bucket, region, endpoint and S3 credentials to your user. Everything under
`/core/v1/{sessionId}/` operates within it. There is no way to address a bucket without one.

Sessions are stored server-side — nothing needs to be held or sent by the browser beyond the
numeric ID in the URL path.

### Create a session

```tsx
interface Session {
  id: number;
  bucket: string;
  nickname: string;
  region: string;
  endpoint: string;
  inserted_at: string;
  updated_at: string;
}

async function createSession(params: {
  bucket: string;
  nickname?: string;
  region: string;
  endpoint: string;
  access_key_id?: string;
  secret_access_key?: string;
}): Promise<Session> {
  const res = await fetch(`${API_URL}/core/v1/session`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json", ...csrfHeaders() },
    body: JSON.stringify(params),
  });

  if (!res.ok) throw new Error("failed to create session");
  const data = await res.json();
  return data.data;
}
```

Credentials are encrypted at rest and never returned — the session response carries only the
public fields listed above.

### Use a session

```tsx
async function listObjects(
  sessionId: number,
  prefix?: string,
): Promise<object[]> {
  const url = new URL(`${API_URL}/core/v1/${sessionId}/objects`);
  if (prefix) url.searchParams.set("prefix", prefix);

  const res = await fetch(url.toString(), { credentials: "include" });

  if (res.status === 400) throw new Error("invalid session ID");
  if (res.status === 404) throw new Error("session not found");
  if (res.status === 403) throw new Error("session belongs to another user");
  if (!res.ok) throw new Error("failed to list objects");
  const data = await res.json();
  return data.data;
}
```

> **Do not send a `bucket` parameter to a session-scoped endpoint.** The bucket always comes
> from the session; handlers overwrite any caller-supplied value, so it is silently ignored.

### List sessions

```tsx
async function listSessions(): Promise<Session[]> {
  const res = await fetch(`${API_URL}/core/v1/sessions`, {
    credentials: "include",
  });
  if (!res.ok) throw new Error("failed to fetch sessions");
  const data = await res.json();
  return data.data;
}
```

Only sessions owned by the authenticated user are returned.

---

## API request helper

One place to attach credentials, CSRF and the refresh-on-401 retry.

```tsx
// lib/api.ts
const API_URL = import.meta.env.VITE_API_URL || "https://api.openbucket.appleby.cloud";

let refreshPromise: Promise<boolean> | null = null;

// Deduplicate concurrent refreshes — a page that fires five requests at once
// must not fire five refreshes.
function refreshTokens(): Promise<boolean> {
  refreshPromise ??= fetch(`${API_URL}/auth/refresh`, {
    method: "POST",
    credentials: "include",
  })
    .then((r) => r.ok)
    .catch(() => false)
    .finally(() => {
      refreshPromise = null;
    });
  return refreshPromise;
}

export async function api<T>(
  endpoint: string,
  options: RequestInit = {},
  retry = true,
): Promise<T> {
  const res = await fetch(`${API_URL}${endpoint}`, {
    ...options,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...csrfHeaders(),
      ...(options.headers as Record<string, string>),
    },
  });

  if (res.status === 401 && retry) {
    if (await refreshTokens()) return api<T>(endpoint, options, false);
    window.location.href = "/login";
    throw new Error("unauthorized");
  }

  if (!res.ok) {
    const err = await res
      .json()
      .catch(() => ({ error_message: "request failed" }));
    throw new Error(err.error_message || "request failed");
  }

  return res.json();
}

// Usage:
// const user = await api<{ data: User }>('/auth/self');
// const objects = await api<{ data: object[] }>('/core/v1/42/objects');
```

---

## React integration

### Auth context

```tsx
// contexts/AuthContext.tsx
import {
  createContext,
  useContext,
  useEffect,
  useState,
  ReactNode,
} from "react";

interface AuthContextType {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  isPending: boolean;
  loginLocal: (email: string, password: string) => Promise<void>;
  loginSSO: () => void;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const checkAuth = async () => {
    try {
      setUser(await getCurrentUser());
    } catch {
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    checkAuth();
  }, []);

  const loginLocal = async (email: string, password: string) => {
    setUser(await login(email, password));
  };

  const loginSSO = () => {
    sessionStorage.setItem("returnUrl", window.location.pathname);
    window.location.href = `${API_URL}/auth/sso/login`;
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading,
        isAuthenticated: !!user,
        isPending: user?.role === "pending",
        loginLocal,
        loginSSO,
        logout,
        refresh: checkAuth,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within an AuthProvider");
  return ctx;
}
```

### Protected route

Handle `pending` separately from unauthenticated — a freshly SSO-provisioned user *is*
authenticated, they just have no access yet.

```tsx
// components/ProtectedRoute.tsx
import { Navigate, useLocation } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext";

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isPending, isLoading } = useAuth();
  const location = useLocation();

  if (isLoading) return <div>Loading…</div>;

  if (!isAuthenticated) {
    sessionStorage.setItem("returnUrl", location.pathname);
    return <Navigate to="/login" replace />;
  }

  if (isPending) return <Navigate to="/pending" replace />;

  return <>{children}</>;
}
```

---

## Handling errors

### Handling 401

The access token expired or the credential is invalid. `POST /auth/refresh` and retry once; if
the refresh also fails, the refresh token is gone too — send the user to login. The helper above
does this automatically.

For an SSO user a 401 can also mean the provider reported the grant inactive during a
checkpoint, in which case refresh will fail and re-login is genuinely required.

### 403 with error code 4004

`your account is pending admin approval`. The user is authenticated but has role `pending`.
Only `/auth/self` works — show a holding page rather than a login prompt.

### 403 with error code 4030 / 4031

Missing or mismatched CSRF token. Check that `X-CSRF-Token` is being sent on the mutation.

### 403 `admin access required`

Authenticated, wrong role. Distinct from `401 authentication required`.

---

## Post-login redirect

After SSO login the API redirects to `OB_SSO_POST_LOGIN_URL`. To send users back where they
were, stash the path before redirecting and restore it on arrival:

```tsx
// pages/Home.tsx
useEffect(() => {
  if (!isLoading && isAuthenticated) {
    const returnUrl = sessionStorage.getItem("returnUrl");
    if (returnUrl) {
      sessionStorage.removeItem("returnUrl");
      navigate(returnUrl, { replace: true });
    }
  }
}, [isAuthenticated, isLoading, navigate]);
```

---

## CORS

Your frontend origin must be in the API's allowlist (`main.go`). Currently:

- `https://openbucket.local.appleby.cloud:3010` (local dev)
- `https://openbucket.appleby.cloud` (production)

`AllowCredentials` is on, and `X-CSRF-Token` is in the allowed-headers list — a new custom
header would need adding there too.

## Cookies across subdomains

For cookies to work across e.g. `openbucket.appleby.cloud` and `api.openbucket.appleby.cloud`:

1. Set `OB_COOKIE_DOMAIN=.appleby.cloud` on the API
2. Both frontend and API must use HTTPS, unless `OB_COOKIE_INSECURE=true` for local dev

Cookies are `SameSite=Lax`, so a cross-site (not merely cross-subdomain) frontend will not
receive them.

---

## Testing locally

1. Start the API with `OB_COOKIE_INSECURE=true` if serving over HTTP
2. Add your local frontend URL to the CORS allowlist in `main.go`
3. Use the same parent domain (e.g. `*.local.appleby.cloud`) so cookies are shared
4. Check DevTools → Application → Cookies for `ob-access-token`, `ob-refresh-token`,
   `ob-logged-in` and `ob-csrf`

---

## TypeScript types

```tsx
// types/api.ts

export interface User {
  id: number;
  email: string;
  name: string | null;
  auth_type: "local" | "sso";
  sso_subject?: string;
  profile_image_url: string | null;
  role: "admin" | "editor" | "viewer" | "pending";
  active: boolean;
  inserted_at: string;
  updated_at: string;
}

export interface Session {
  id: number;
  bucket: string;
  nickname: string;
  region: string;
  endpoint: string;
  inserted_at: string;
  updated_at: string;
}

export interface ApiSuccess<T> {
  success: true;
  message: string;
  data: T;
}

export interface ApiError {
  error: string | null;
  error_message: string;
  error_code: number;
}
```

---

## Checklist

- [ ] Frontend URL added to the API CORS allowlist
- [ ] `credentials: "include"` on every fetch
- [ ] `X-CSRF-Token` sent on every cookie-authenticated mutation
- [ ] SSO button rendered only when `GET /auth/sso/config` returns `enabled: true`
- [ ] Local login posts to `/auth/login`; tokens read from cookies, not the body
- [ ] Logout is a `POST` to `/auth/logout`, not a navigation
- [ ] Auth state determined by `GET /auth/self` (200 = authenticated, 401 = not)
- [ ] 401 triggers a single deduplicated `POST /auth/refresh` and one retry
- [ ] `pending` role handled distinctly from unauthenticated (403 / error code 4004)
- [ ] Sessions created via `POST /core/v1/session`, listed via `GET /core/v1/sessions`
- [ ] Session ID included in the bucket route path (`/core/v1/{sessionId}/…`)
- [ ] No `bucket` parameter sent to session-scoped endpoints — it is ignored
- [ ] `400`/`403`/`404` on bucket requests handled distinctly
- [ ] `OB_COOKIE_DOMAIN` configured for cross-subdomain use
