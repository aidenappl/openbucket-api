# Frontend Forta Integration Guide

This guide explains how to integrate Forta authentication into your frontend application using the OpenBucket API.

---

## Overview

Forta uses cookie-based OAuth2 authentication. The flow is:

1. Frontend navigates to the API's `/forta/login` endpoint
2. API generates CSRF token, sets state cookie, and redirects to `login.appleby.cloud`
3. User authenticates on Forta
4. Forta redirects back to the configured `FORTA_CALLBACK_URL` (handled server-side)
5. API exchanges code for tokens and sets HttpOnly cookies
6. API redirects to `FORTA_POST_LOGIN_REDIRECT`
7. Subsequent requests automatically include auth cookies

**Important:** There is no public `/forta/callback` or `/forta/check` endpoint. Authentication state is determined by calling `/self` — a `401` means unauthenticated, a `200` means authenticated.

---

## Quick Start

### 1. Add Login Button

```tsx
function LoginButton() {
  const handleLogin = () => {
    window.location.href = "https://your-api.com/forta/login";
  };

  return <button onClick={handleLogin}>Sign in with Forta</button>;
}
```

### 2. Add Logout Button

```tsx
function LogoutButton() {
  const handleLogout = () => {
    window.location.href = "https://your-api.com/forta/logout";
  };

  return <button onClick={handleLogout}>Sign out</button>;
}
```

### 3. Get Current User (Protected)

```tsx
interface User {
  id: number;
  email: string;
  name: string | null;
  display_name: string | null;
}

async function getCurrentUser(): Promise<User | null> {
  const response = await fetch("https://your-api.com/self", {
    credentials: "include",
  });

  if (response.status === 401) {
    return null; // not authenticated
  }

  if (!response.ok) {
    throw new Error("Failed to fetch user");
  }

  const data = await response.json();
  return data.data;
}
```

---

## Sessions

Create a session once to associate your S3 credentials with your Forta user. Sessions are stored server-side and automatically resolved for every bucket request — nothing needs to be stored or sent by the browser.

### Create a Session

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
  const response = await fetch("https://your-api.com/core/v1/session", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(params),
  });

  if (!response.ok) throw new Error("Failed to create session");
  const data = await response.json();
  return data.data;
}
```

### Use a Session

Fetch your sessions first to get session IDs, then include the session ID in the URL path. The API resolves the bucket and credentials from the session ID and verifies ownership against your Forta identity:

```tsx
async function listObjects(
  sessionId: number,
  prefix?: string,
): Promise<object[]> {
  const url = new URL(`https://your-api.com/core/v1/${sessionId}/objects`);
  if (prefix) url.searchParams.set("prefix", prefix);

  const response = await fetch(url.toString(), {
    credentials: "include",
  });

  if (response.status === 400) throw new Error("Invalid session ID");
  if (response.status === 404) throw new Error("Session not found");
  if (response.status === 403)
    throw new Error("Session belongs to another user");
  if (!response.ok) throw new Error("Failed to list objects");
  const data = await response.json();
  return data.data;
}
```

### List All Sessions

```tsx
async function listSessions(): Promise<Session[]> {
  const response = await fetch("https://your-api.com/core/v1/sessions", {
    credentials: "include",
  });

  if (!response.ok) throw new Error("Failed to fetch sessions");
  const data = await response.json();
  return data.data;
}
```

Only sessions owned by the authenticated user are returned.

---

## React Integration Example

### Auth Context

```tsx
// contexts/AuthContext.tsx
import {
  createContext,
  useContext,
  useEffect,
  useState,
  ReactNode,
} from "react";

interface User {
  id: number;
  email: string;
  name: string | null;
  display_name: string | null;
}

interface AuthContextType {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  login: () => void;
  logout: () => void;
  refresh: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

const API_URL = import.meta.env.VITE_API_URL || "https://your-api.com";

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const checkAuth = async () => {
    try {
      const response = await fetch(`${API_URL}/self`, {
        credentials: "include",
      });
      if (response.ok) {
        const data = await response.json();
        setUser(data.data);
      } else {
        setUser(null);
      }
    } catch (error) {
      console.error("Auth check failed:", error);
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    checkAuth();
  }, []);

  const login = () => {
    sessionStorage.setItem("returnUrl", window.location.pathname);
    window.location.href = `${API_URL}/forta/login`;
  };

  const logout = () => {
    window.location.href = `${API_URL}/forta/logout`;
  };

  const refresh = async () => {
    setIsLoading(true);
    await checkAuth();
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading,
        isAuthenticated: !!user,
        login,
        logout,
        refresh,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
```

### Protected Route Component

```tsx
// components/ProtectedRoute.tsx
import { Navigate, useLocation } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext";

interface ProtectedRouteProps {
  children: React.ReactNode;
}

export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const { isAuthenticated, isLoading } = useAuth();
  const location = useLocation();

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (!isAuthenticated) {
    sessionStorage.setItem("returnUrl", location.pathname);
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}
```

### Header Component with Auth

```tsx
// components/Header.tsx
import { useAuth } from "../contexts/AuthContext";

export function Header() {
  const { user, isAuthenticated, isLoading, login, logout } = useAuth();

  return (
    <header>
      <nav>
        <a href="/">Home</a>
        {isLoading ? (
          <span>Loading...</span>
        ) : isAuthenticated ? (
          <>
            <span>Welcome, {user?.display_name || user?.email}</span>
            <button onClick={logout}>Sign out</button>
          </>
        ) : (
          <button onClick={login}>Sign in</button>
        )}
      </nav>
    </header>
  );
}
```

---

## API Request Helper

```tsx
// lib/api.ts
const API_URL = import.meta.env.VITE_API_URL || "https://your-api.com";

export async function api<T>(
  endpoint: string,
  options: RequestInit = {},
): Promise<T> {
  const response = await fetch(`${API_URL}${endpoint}`, {
    ...options,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(options.headers as Record<string, string>),
    },
  });

  if (!response.ok) {
    if (response.status === 401) {
      window.location.href = `${API_URL}/forta/login`;
      throw new Error("Unauthorized");
    }
    const error = await response
      .json()
      .catch(() => ({ error: "Request failed" }));
    throw new Error(error.error || "Request failed");
  }

  return response.json();
}

// Usage examples:
// const user = await api<{ data: User }>('/self');
// const objects = await api<{ data: object[] }>('/core/v1/42/objects');
```

---

## Handling Post-Login Redirect

After successful login the API redirects to `FORTA_POST_LOGIN_REDIRECT` (default: `/`). To redirect users back where they were:

```tsx
// pages/Home.tsx
import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext";

export function Home() {
  const { isAuthenticated, isLoading } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (!isLoading && isAuthenticated) {
      const returnUrl = sessionStorage.getItem("returnUrl");
      if (returnUrl) {
        sessionStorage.removeItem("returnUrl");
        navigate(returnUrl, { replace: true });
      }
    }
  }, [isAuthenticated, isLoading, navigate]);

  return <div>Welcome to OpenBucket</div>;
}
```

---

## CORS Configuration

Your frontend origin must be in the API's CORS allowed origins. The API currently allows:

- `https://openbucket.local.appleby.cloud:3010` (local dev)
- `https://openbucket.appleby.cloud` (production)

If your frontend is hosted elsewhere, update the CORS configuration in `main.go`.

---

## Cookie Configuration

For cookies to work across subdomains (e.g., `openbucket.appleby.cloud` and `api.appleby.cloud`):

1. Set `FORTA_COOKIE_DOMAIN=.appleby.cloud` on the API
2. Both frontend and API must use HTTPS (unless `FORTA_COOKIE_INSECURE=true` for local dev)

---

## Error Handling

### 401 Unauthorized

The user's session has expired or is invalid. Redirect to login:

```tsx
if (response.status === 401) {
  window.location.href = `${API_URL}/forta/login`;
}
```

### Auto-Refresh

The API automatically refreshes expired access tokens using the refresh token cookie. If auto-refresh fails (e.g., refresh token also expired), you'll receive a `401`.

---

## TypeScript Types

```tsx
// types/api.ts

export interface User {
  id: number;
  email: string;
  name: string | null;
  display_name: string | null;
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

export interface ErrorResponse {
  error: string;
}
```

---

## Testing Authentication Locally

1. Start the API with `FORTA_COOKIE_INSECURE=true` for HTTP development
2. Ensure your local frontend URL is in CORS allowed origins
3. Use the same domain pattern (e.g., `*.local.appleby.cloud`) for cookie sharing
4. Check browser DevTools → Application → Cookies to verify tokens are set

---

## Checklist

- [ ] Frontend URL added to API CORS config
- [ ] `credentials: 'include'` on all fetch requests
- [ ] Login button redirects to `/forta/login`
- [ ] Logout button redirects to `/forta/logout`
- [ ] Auth status determined by calling `/self` (200 = authenticated, 401 = not)
- [ ] Sessions created via `POST /core/v1/session`
- [ ] All sessions listed via `GET /core/v1/sessions`
- [ ] Session ID included in bucket route path (`/core/v1/{sessionId}/...`)
- [ ] `404`/`403` on bucket requests handled (session not found or wrong user)
- [ ] Protected routes redirect unauthenticated users
- [ ] 401 errors trigger re-authentication
- [ ] Cookie domain configured for cross-subdomain (if needed)
