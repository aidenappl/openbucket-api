package routers

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/aidenappl/go-forta/activity"
	monitor "github.com/aidenappl/go-monitor"
	"github.com/aidenappl/openbucket-api/env"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/sso"
	"github.com/gorilla/mux"
)

// Activity sink: successful changes an SSO-signed-in person makes in OpenBucket
// (uploads, deletes, renames, ACL changes, folder and admin changes) are
// mirrored to an external sink — the identity provider's per-user activity
// trail — so "everything this person did" spans platforms.
//
// Opt-in: off unless OB_ACTIVITY_REPORT_URL is set. OpenBucket has no audit
// table, so this is one middleware keyed on the matched route rather than a
// hook per handler: a new mutating route is reported (generically) without a
// change here. Fire-and-forget; never fails or slows a request. Only
// identifiers travel — bucket, object key, folder, record id — never content,
// tokens or request bodies.

var (
	sinkMu       sync.Mutex
	sinkReporter *activity.Reporter
	sinkClient   string // ClientID + "\x00" + ClientSecret the reporter was built with
)

// activitySink returns a reporter for the current SSO client, or nil when no
// sink is configured or SSO cannot authenticate to it. SSO config is
// DB-backed and editable at runtime, so the reporter follows the client.
func activitySink() *activity.Reporter {
	if env.ActivityReportURL == "" {
		return nil
	}
	cfg := sso.LoadConfig()
	if cfg == nil || !cfg.Enabled || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil
	}
	client := cfg.ClientID + "\x00" + cfg.ClientSecret
	sinkMu.Lock()
	defer sinkMu.Unlock()
	if sinkReporter != nil && client == sinkClient {
		return sinkReporter
	}
	if old := sinkReporter; old != nil {
		go old.Close(context.Background())
	}
	sinkReporter = activity.New(activity.Config{
		Endpoint:     env.ActivityReportURL,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		OnError: func(action string, err error) {
			monitor.Warn(context.Background(), "activity_sink.report.failed", map[string]any{"action": action, "error": err.Error()})
		},
	})
	sinkClient = client
	return sinkReporter
}

// sinkActions names a mutating route. Key: method + " " + the route template
// with the /core/v1/{sessionId} or /admin prefix removed. Routes not listed
// are reported as "<first path segment>.<method>".
var sinkActions = map[string]string{
	"POST /session":                     "session.create",
	"DELETE /session/{id}":              "session.delete",
	"PUT /object":                       "object.upload",
	"DELETE /object":                    "object.delete",
	"PUT /object/rename":                "object.rename",
	"PUT /object/acl":                   "object.acl_change",
	"POST /object/acl":                  "object.acl_change",
	"POST /folder":                      "folder.create",
	"PUT /folder":                       "folder.update",
	"DELETE /folder":                    "folder.delete",
	"POST /users":                       "user.create",
	"PUT /users/{id}":                   "user.update",
	"DELETE /users/{id}":                "user.delete",
	"POST /api-tokens":                  "api_token.create",
	"DELETE /api-tokens/{id}":           "api_token.revoke",
	"PUT /sso-config":                   "sso_config.update",
	"POST /instances":                   "instance.create",
	"PUT /instances/{id}":               "instance.update",
	"DELETE /instances/{id}":            "instance.delete",
	"POST /instances/{id}/proxy/{path}": "instance.admin_change",
	"PUT /instances/{id}/proxy/{path}":  "instance.admin_change",
	// DELETE through the proxy is a change too.
	"DELETE /instances/{id}/proxy/{path}": "instance.admin_change",
}

// readOnlyPosts are POSTs that change nothing.
var readOnlyPosts = map[string]bool{"/object/head": true}

// sinkRoute strips the subrouter prefix from a route template and normalises
// {path:.*}-style variables to {path}.
func sinkRoute(tpl string) string {
	for _, p := range []string{"/core/v1/{sessionId}", "/core/v1", "/admin"} {
		if strings.HasPrefix(tpl, p+"/") {
			tpl = strings.TrimPrefix(tpl, p)
			break
		}
	}
	if i := strings.Index(tpl, "{path:"); i >= 0 {
		tpl = tpl[:i] + "{path}"
	}
	return tpl
}

// sinkAction maps a request to an action, or "" for reads.
func sinkAction(method, route string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return ""
	}
	if method == http.MethodPost && readOnlyPosts[route] {
		return ""
	}
	if a, ok := sinkActions[method+" "+route]; ok {
		return a
	}
	seg := strings.Trim(route, "/")
	if i := strings.IndexByte(seg, '/'); i >= 0 {
		seg = seg[:i]
	}
	if seg == "" {
		return ""
	}
	return strings.ReplaceAll(seg, "-", "_") + "." + strings.ToLower(method)
}

// sinkWriter records the status; Unwrap keeps http.ResponseController
// (LongTransfer's deadlines, flushing) working through it.
type sinkWriter struct {
	http.ResponseWriter
	status int
}

func (w *sinkWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *sinkWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *sinkWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// ActivitySink returns middleware reporting successful (2xx) mutating requests
// by an SSO user. Mount it after the auth middleware so the user is in
// context. Parent-router middleware also wraps subrouter routes, so it is
// mounted twice: on `core`/`admin` with bucketScoped=false (routes without
// {sessionId}) and on the bucket subrouter, after SessionMiddleware, with
// bucketScoped=true — the only place the session (and so the bucket) is in
// context. Each instance ignores the other's routes, so nothing is reported
// twice.
func ActivitySink(bucketScoped bool) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return activitySinkHandler(next, bucketScoped)
	}
}

func activitySinkHandler(next http.Handler, bucketScoped bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if env.ActivityReportURL == "" {
			next.ServeHTTP(w, r)
			return
		}
		tpl := ""
		if cr := mux.CurrentRoute(r); cr != nil {
			tpl, _ = cr.GetPathTemplate()
		}
		if strings.Contains(tpl, "{sessionId}") != bucketScoped {
			next.ServeHTTP(w, r)
			return
		}
		action := sinkAction(r.Method, sinkRoute(tpl))
		if action == "" {
			next.ServeHTTP(w, r)
			return
		}
		sw := &sinkWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if sw.status < 200 || sw.status > 299 {
			return
		}
		user, ok := middleware.GetUserFromContext(r.Context())
		if !ok || user == nil || user.SSOSubject == nil || *user.SSOSubject == "" || sso.LooksLikeEmail(*user.SSOSubject) {
			return
		}
		sinkReport(*user.SSOSubject, action, sinkDetails(r))
	})
}

// sinkReport hands one event to the reporter off the request path. A variable
// so tests can observe what would be sent without a database or sink.
var sinkReport = func(sub, action string, details map[string]string) {
	go func() {
		if rep := activitySink(); rep != nil {
			rep.Report(sub, action, activity.Success, details)
		}
	}()
}

// sinkDetails collects identifiers only.
func sinkDetails(r *http.Request) map[string]string {
	d := map[string]string{}
	if s := middleware.GetSession(r.Context()); s != nil && s.BucketName != "" {
		d["bucket"] = s.BucketName
	}
	q := r.URL.Query()
	for _, k := range []string{"key", "newKey", "folder"} {
		if v := q.Get(k); v != "" {
			d[strings.ToLower(k)] = v
		}
	}
	if id := mux.Vars(r)["id"]; id != "" {
		d["id"] = id
	}
	if strings.Contains(r.URL.Path, "/proxy/") {
		d["method"] = r.Method
	}
	return d
}
