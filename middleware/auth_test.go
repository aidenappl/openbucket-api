package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aidenappl/openbucket-api/structs"
)

func TestGetUserFromContext_Present(t *testing.T) {
	user := &structs.User{ID: 42, Email: "test@example.com", Role: "admin", Active: true}
	ctx := context.WithValue(context.Background(), UserContextKey, user)

	got, ok := GetUserFromContext(ctx)
	if !ok {
		t.Fatal("expected user to be present in context")
	}
	if got.ID != 42 {
		t.Fatalf("expected user ID 42, got %d", got.ID)
	}
}

func TestGetUserFromContext_Missing(t *testing.T) {
	_, ok := GetUserFromContext(context.Background())
	if ok {
		t.Fatal("expected user not to be present in empty context")
	}
}

func TestGetUserID_Present(t *testing.T) {
	user := &structs.User{ID: 99, Email: "test@example.com", Active: true}
	ctx := context.WithValue(context.Background(), UserContextKey, user)

	id, ok := GetUserID(ctx)
	if !ok {
		t.Fatal("expected user ID to be present")
	}
	if id != 99 {
		t.Fatalf("expected 99, got %d", id)
	}
}

func TestGetUserID_Missing(t *testing.T) {
	id, ok := GetUserID(context.Background())
	if ok {
		t.Fatal("expected user ID not to be present")
	}
	if id != 0 {
		t.Fatalf("expected 0, got %d", id)
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		want       string
	}{
		{"valid", "Bearer mytoken123", "mytoken123"},
		{"case insensitive", "bearer mytoken123", "mytoken123"},
		{"empty header", "", ""},
		{"no scheme", "mytoken123", ""},
		{"basic auth", "Basic dXNlcjpwYXNz", ""},
		{"bearer no space", "Bearertoken", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			got := extractBearerToken(req)
			if got != tt.want {
				t.Errorf("extractBearerToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRejectPending(t *testing.T) {
	handler := RejectPending(http.HandlerFunc(okHandler))

	t.Run("pending user blocked", func(t *testing.T) {
		user := &structs.User{ID: 1, Role: "pending", Active: true}
		ctx := context.WithValue(context.Background(), UserContextKey, user)
		req := httptest.NewRequest(http.MethodGet, "/buckets", nil).WithContext(ctx)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for pending user, got %d", rr.Code)
		}
	})

	t.Run("active user passes", func(t *testing.T) {
		user := &structs.User{ID: 1, Role: "viewer", Active: true}
		ctx := context.WithValue(context.Background(), UserContextKey, user)
		req := httptest.NewRequest(http.MethodGet, "/buckets", nil).WithContext(ctx)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for viewer user, got %d", rr.Code)
		}
	})

	t.Run("admin user passes", func(t *testing.T) {
		user := &structs.User{ID: 1, Role: "admin", Active: true}
		ctx := context.WithValue(context.Background(), UserContextKey, user)
		req := httptest.NewRequest(http.MethodGet, "/buckets", nil).WithContext(ctx)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for admin user, got %d", rr.Code)
		}
	})
}

func TestRequireAdmin(t *testing.T) {
	handler := RequireAdmin(okHandler)

	t.Run("admin passes", func(t *testing.T) {
		user := &structs.User{ID: 1, Role: "admin", Active: true}
		ctx := context.WithValue(context.Background(), UserContextKey, user)
		req := httptest.NewRequest(http.MethodGet, "/admin/users", nil).WithContext(ctx)
		rr := httptest.NewRecorder()

		handler(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("viewer blocked", func(t *testing.T) {
		user := &structs.User{ID: 1, Role: "viewer", Active: true}
		ctx := context.WithValue(context.Background(), UserContextKey, user)
		req := httptest.NewRequest(http.MethodGet, "/admin/users", nil).WithContext(ctx)
		rr := httptest.NewRecorder()

		handler(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("editor blocked", func(t *testing.T) {
		user := &structs.User{ID: 1, Role: "editor", Active: true}
		ctx := context.WithValue(context.Background(), UserContextKey, user)
		req := httptest.NewRequest(http.MethodGet, "/admin/users", nil).WithContext(ctx)
		rr := httptest.NewRecorder()

		handler(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("no user in context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
		rr := httptest.NewRecorder()

		handler(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})
}

func TestRequireEditor(t *testing.T) {
	handler := RequireEditor(okHandler)

	t.Run("admin passes", func(t *testing.T) {
		user := &structs.User{ID: 1, Role: "admin", Active: true}
		ctx := context.WithValue(context.Background(), UserContextKey, user)
		req := httptest.NewRequest(http.MethodPost, "/upload", nil).WithContext(ctx)
		rr := httptest.NewRecorder()

		handler(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("editor passes", func(t *testing.T) {
		user := &structs.User{ID: 1, Role: "editor", Active: true}
		ctx := context.WithValue(context.Background(), UserContextKey, user)
		req := httptest.NewRequest(http.MethodPost, "/upload", nil).WithContext(ctx)
		rr := httptest.NewRecorder()

		handler(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("viewer blocked", func(t *testing.T) {
		user := &structs.User{ID: 1, Role: "viewer", Active: true}
		ctx := context.WithValue(context.Background(), UserContextKey, user)
		req := httptest.NewRequest(http.MethodPost, "/upload", nil).WithContext(ctx)
		rr := httptest.NewRecorder()

		handler(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("no user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/upload", nil)
		rr := httptest.NewRecorder()

		handler(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})
}
