package routers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aidenappl/openbucket-api/env"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/structs"
	"github.com/aidenappl/openbucket-api/tools"
	"github.com/gorilla/mux"
)

func TestSinkAction(t *testing.T) {
	cases := []struct{ method, tpl, want string }{
		{http.MethodPut, "/core/v1/{sessionId}/object", "object.upload"},
		{http.MethodDelete, "/core/v1/{sessionId}/object", "object.delete"},
		{http.MethodPost, "/core/v1/{sessionId}/object/acl", "object.acl_change"},
		{http.MethodPost, "/core/v1/{sessionId}/object/head", ""}, // bulk HEAD is a read
		{http.MethodGet, "/core/v1/{sessionId}/object", ""},
		{http.MethodPost, "/core/v1/session", "session.create"},
		{http.MethodDelete, "/core/v1/session/{id}", "session.delete"},
		{http.MethodDelete, "/admin/api-tokens/{id}", "api_token.revoke"},
		{http.MethodPut, "/admin/instances/{id}/proxy/{path:.*}", "instance.admin_change"},
		{http.MethodPatch, "/admin/new-thing/{id}", "new_thing.patch"}, // unlisted: generic
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.tpl, func(t *testing.T) {
			if got := sinkAction(c.method, sinkRoute(c.tpl)); got != c.want {
				t.Errorf("got %q want %q", got, c.want)
			}
		})
	}
}

// Mounted as main.go mounts it, each request is reported exactly once, with
// the bucket for bucket routes, and only for SSO users and 2xx responses.
func TestActivitySinkMountedTwiceReportsOnce(t *testing.T) {
	prevURL := env.ActivityReportURL
	env.ActivityReportURL = "https://sink.example.test/internal/activity"
	defer func() { env.ActivityReportURL = prevURL }()

	type event struct {
		sub, action string
		details     map[string]string
	}
	var got []event
	prev := sinkReport
	sinkReport = func(sub, action string, details map[string]string) { got = append(got, event{sub, action, details}) }
	defer func() { sinkReport = prev }()

	sub, email := "uuid-1", "a@b.test"
	user := &structs.User{ID: 1, SSOSubject: &sub}
	withUser := func(u *structs.User) mux.MiddlewareFunc {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), middleware.UserContextKey, u)))
			})
		}
	}
	withSession := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), middleware.SessionContextKey, &tools.SessionClaims{BucketName: "photos"})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
	status := http.StatusOK
	h := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }

	build := func(u *structs.User) *mux.Router {
		r := mux.NewRouter()
		core := r.PathPrefix("/core/v1/").Subrouter()
		core.Use(withUser(u), ActivitySink(false))
		core.HandleFunc("/session", h).Methods(http.MethodPost)
		bucket := core.PathPrefix("/{sessionId}").Subrouter()
		bucket.Use(withSession, ActivitySink(true))
		bucket.HandleFunc("/object", h).Methods(http.MethodPut, http.MethodGet)
		return r
	}
	do := func(r *mux.Router, method, path string) {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, path, nil))
	}

	r := build(user)
	do(r, http.MethodPut, "/core/v1/abc/object?key=a/b.png")
	do(r, http.MethodPost, "/core/v1/session")
	do(r, http.MethodGet, "/core/v1/abc/object?key=a/b.png") // read
	status = http.StatusForbidden
	do(r, http.MethodPut, "/core/v1/abc/object?key=denied.png") // not 2xx
	status = http.StatusOK
	do(build(&structs.User{ID: 2}), http.MethodPut, "/core/v1/abc/object?key=x")                     // local user
	do(build(&structs.User{ID: 3, SSOSubject: &email}), http.MethodPut, "/core/v1/abc/object?key=x") // pre-heal subject

	if len(got) != 2 {
		t.Fatalf("got %d events, want 2: %+v", len(got), got)
	}
	if got[0].action != "object.upload" || got[0].sub != sub || got[0].details["bucket"] != "photos" || got[0].details["key"] != "a/b.png" {
		t.Errorf("upload event = %+v", got[0])
	}
	if got[1].action != "session.create" {
		t.Errorf("session event = %+v", got[1])
	}
}
