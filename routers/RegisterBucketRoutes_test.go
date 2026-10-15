package routers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/structs"
	"github.com/gorilla/mux"
)

// bucketRoutes classifies every session-scoped route. The completeness check
// in TestBucketRoutesEnforceRoles fails when a route is registered without
// being listed here, so a new mutating route can't skip RequireEditor quietly.
var bucketRoutes = []struct {
	method string
	path   string
	write  bool
}{
	{http.MethodPut, "/object", true},
	{http.MethodGet, "/object", false},
	{http.MethodGet, "/object/head", false},
	{http.MethodPost, "/object/head", false}, // bulk HEAD
	{http.MethodGet, "/object/acl", false},
	{http.MethodPut, "/object/acl", true},
	{http.MethodPost, "/object/acl", true}, // bulk ACL
	{http.MethodDelete, "/object", true},
	{http.MethodGet, "/objects", false},
	{http.MethodGet, "/object/presign", false},
	{http.MethodPut, "/object/rename", true},
	{http.MethodGet, "/folder", false},
	{http.MethodGet, "/folders", false},
	{http.MethodGet, "/contents", false},
	{http.MethodPost, "/folder", true},
	{http.MethodPut, "/folder", true},
	{http.MethodDelete, "/folder", true},
}

// routerAs mounts the bucket routes under /core/v1/{sessionId} with `role`
// already in the context — standing in for AuthMiddleware. SessionMiddleware
// is left out on purpose: a handler that runs answers "session not found",
// which is distinguishable from the role check's 403.
func routerAs(role string) *mux.Router {
	r := mux.NewRouter()
	sub := r.PathPrefix("/core/v1/{sessionId}").Subrouter()
	sub.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			user := &structs.User{ID: 1, Email: role + "@example.com", Role: role, Active: true}
			next.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, user)))
		})
	})
	RegisterBucketRoutes(sub)
	return r
}

// blockedByRole reports whether the request was stopped by RequireEditor.
func blockedByRole(r *mux.Router, method, path string) bool {
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(method, "/core/v1/1"+path, strings.NewReader("{}")))
	return rr.Code == http.StatusForbidden && strings.Contains(rr.Body.String(), "editor access required")
}

func TestBucketRoutesEnforceRoles(t *testing.T) {
	for _, role := range []string{"viewer", "editor", "admin"} {
		r := routerAs(role)
		for _, rt := range bucketRoutes {
			t.Run(role+" "+rt.method+" "+rt.path, func(t *testing.T) {
				blocked := blockedByRole(r, rt.method, rt.path)
				want := role == "viewer" && rt.write
				if blocked != want {
					t.Fatalf("%s %s %s: blocked=%v, want %v", role, rt.method, rt.path, blocked, want)
				}
			})
		}
	}

	t.Run("every registered route is classified", func(t *testing.T) {
		known := map[string]bool{}
		for _, rt := range bucketRoutes {
			known[rt.method+" "+rt.path] = true
		}
		r := mux.NewRouter()
		RegisterBucketRoutes(r)
		err := r.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
			tpl, err := route.GetPathTemplate()
			if err != nil {
				return nil
			}
			methods, err := route.GetMethods()
			if err != nil {
				return nil
			}
			for _, m := range methods {
				if !known[m+" "+tpl] {
					t.Errorf("route %s %s is not in bucketRoutes — classify it as read or write", m, tpl)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}
