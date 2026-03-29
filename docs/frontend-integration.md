# Frontend Forta Integration Guide

This guide explains how to integrate Forta authentication into your frontend application using the OpenBucket API.

---

## Overview

Forta uses cookie-based OAuth2 authentication. The flow is:

1. Frontend navigates to the API's `/forta/login` endpoint
2. API generates CSRF token, sets state cookie, and redirects to `login.appleby.cloud`
3. User authenticates on Forta
4. Forta redirects back to `/forta/callback` with auth code
5. API exchanges code for tokens and sets HttpOnly cookies
6. API redirects to your app
7. Subsequent requests automatically include auth cookies

**Important:** Always use the API's `/forta/login` endpoint rather than redirecting directly to `login.appleby.cloud`. The API generates the proper OAuth2 URL with CSRF protection.

---

## Quick Start

### 1. Add Login Button

```tsx
function LoginButton() {
  const handleLogin = () => {
    // Navigate to the API's login endpoint
    // The API will redirect to login.appleby.cloud with the proper OAuth2 URL
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

### 3. Check Authentication Status

```tsx
interface User {
  id: number;
  email: string;
  name: string | null;
  display_name: string | null;
}

interface AuthCheckResponse {
  authenticated: boolean;
  user?: User;
  message?: string;
}

async function checkAuth(): Promise<AuthCheckResponse> {
  const response = await fetch("https://your-api.com/forta/check", {
    credentials: "include", // IMPORTANT: Include cookies
  });
  return response.json();
}
```

### 4. Get Current User (Protected)

```tsx
async function getCurrentUser(): Promise<User> {
  const response = await fetch("https://your-api.com/self", {
    credentials: "include",
  });

  if (!response.ok) {
    if (response.status === 401) {
      throw new Error("Not authenticated");
    }
    throw new Error("Failed to fetch user");
  }

  const data = await response.json();
  return data.data;
}
```

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
      const response = await fetch(`${API_URL}/forta/check`, {
        credentials: "include",
      });
      const data = await response.json();

      if (data.authenticated && data.user) {
        setUser(data.user);
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
    // Store current URL to redirect back after login
    sessionStorage.setItem("returnUrl", window.location.pathname);
    // Navigate to API's login endpoint - it will redirect to login.appleby.cloud
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
    return <div>Loading...</div>; // Or your loading spinner
  }

  if (!isAuthenticated) {
    // Save the attempted URL for redirecting after login
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

### App Setup

```tsx
// App.tsx
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { AuthProvider } from "./contexts/AuthContext";
import { ProtectedRoute } from "./components/ProtectedRoute";
import { Header } from "./components/Header";

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Header />
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/login" element={<LoginPage />} />
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <Dashboard />
              </ProtectedRoute>
            }
          />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}
```

---

## API Request Helper

When making authenticated API requests, always include credentials:

```tsx
// lib/api.ts
const API_URL = import.meta.env.VITE_API_URL || "https://your-api.com";

interface ApiOptions extends RequestInit {
  // Add any custom options here
}

export async function api<T>(
  endpoint: string,
  options: ApiOptions = {},
): Promise<T> {
  const response = await fetch(`${API_URL}${endpoint}`, {
    ...options,
    credentials: "include", // Always include cookies
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
  });

  if (!response.ok) {
    if (response.status === 401) {
      // Optionally redirect to login or refresh auth
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
// const objects = await api<{ data: Object[] }>('/core/v1/mybucket/objects');
```

---

## Handling Post-Login Redirect

After successful login, the API redirects to `FORTA_POST_LOGIN_REDIRECT` (default: `/`). To redirect users back to where they were:

### Option 1: Configure Server-Side

Set `FORTA_POST_LOGIN_REDIRECT` to a specific page like `/auth/callback` that handles the redirect:

```tsx
// pages/AuthCallback.tsx
import { useEffect } from "react";
import { useNavigate } from "react-router-dom";

export function AuthCallback() {
  const navigate = useNavigate();

  useEffect(() => {
    const returnUrl = sessionStorage.getItem("returnUrl") || "/";
    sessionStorage.removeItem("returnUrl");
    navigate(returnUrl, { replace: true });
  }, [navigate]);

  return <div>Completing sign in...</div>;
}
```

### Option 2: Check Auth on Landing Page

If redirecting to `/`, check auth status and handle accordingly:

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
  // Session expired - redirect to login
  window.location.href = `${API_URL}/forta/login`;
}
```

### Auto-Refresh

The API automatically refreshes expired access tokens using the refresh token cookie. This is transparent to the frontend. If auto-refresh fails (e.g., refresh token also expired), you'll receive a 401.

---

## TypeScript Types

```tsx
// types/auth.ts

export interface User {
  id: number;
  email: string;
  name: string | null;
  display_name: string | null;
}

export interface AuthCheckResponse {
  authenticated: boolean;
  user?: User;
  message?: string;
}

export interface CurrentUserResponse {
  data: User;
  message: string;
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
- [ ] Auth check on app load via `/forta/check`
- [ ] Protected routes redirect unauthenticated users
- [ ] 401 errors trigger re-authentication
- [ ] Cookie domain configured for cross-subdomain (if needed)
