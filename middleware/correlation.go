package middleware

import (
	"context"
	"log"
	"net/http"
	"regexp"
	"strings"

	ssolib "github.com/aidenappl/go-forta/sso"
)

// correlationContextKey carries this request's correlation ids so log lines
// written on its behalf (the SSO checkpoint's LogfCtx) can name them. go-forta
// keeps its own copy under an unexported key for the outbound introspection.
const correlationContextKey contextKey = "correlation"

type correlationIDs struct {
	RequestID string
	TraceID   string
}

// correlationIDPattern is the id shape go-forta forwards and monitor-core
// accepts: a UUID or 8-64 hex characters. Anything else is dropped — inbound
// X-Request-ID is caller-supplied, and must never reach a log line unvalidated.
var correlationIDPattern = regexp.MustCompile(`^(?:[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}|[0-9a-fA-F]{8,64})$`)

func validCorrelationID(id string) string {
	id = strings.TrimSpace(id)
	if !correlationIDPattern.MatchString(id) {
		return ""
	}
	return id
}

// traceparentPattern is a W3C Trace Context traceparent:
// version-traceid-parentid-flags, all lowercase hex.
var traceparentPattern = regexp.MustCompile(`^([0-9a-f]{2})-([0-9a-f]{32})-([0-9a-f]{16})-([0-9a-f]{2})(?:-.*)?$`)

// traceIDFrom returns the trace id from a valid W3C traceparent header, else
// the X-Trace-ID header, else "".
func traceIDFrom(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("traceparent"))
	if m := traceparentPattern.FindStringSubmatch(h); m != nil {
		version, traceID, parentID := m[1], m[2], m[3]
		// Version ff is forbidden, version 00 allows no trailing fields, and
		// all-zero trace or parent ids are invalid.
		validVersion := version != "ff" && (version != "00" || len(h) == 55)
		if validVersion && traceID != strings.Repeat("0", 32) && parentID != strings.Repeat("0", 16) {
			return traceID
		}
	}
	return strings.TrimSpace(r.Header.Get("X-Trace-ID"))
}

// correlatedContext returns r's context carrying the inbound request and trace
// ids, both for go-forta (forwarded on SSO introspection to the IdP) and for
// this service's own checkpoint log lines. Invalid ids are dropped.
func correlatedContext(r *http.Request) context.Context {
	rid := validCorrelationID(r.Header.Get("X-Request-ID"))
	tid := validCorrelationID(traceIDFrom(r))
	ctx := ssolib.WithCorrelation(r.Context(), rid, tid)
	if rid == "" && tid == "" {
		return ctx
	}
	return context.WithValue(ctx, correlationContextKey, correlationIDs{RequestID: rid, TraceID: tid})
}

// correlationFrom returns the ids stored by correlatedContext, if any.
func correlationFrom(ctx context.Context) (requestID, traceID string) {
	ids, _ := ctx.Value(correlationContextKey).(correlationIDs)
	return ids.RequestID, ids.TraceID
}

// logfCtx is the SSO checkpoint's LogfCtx: the stdlib log.Printf this service
// uses everywhere, with the request's ids appended when present. go-forta's
// checkpoint lines name a user id and provider slug, never a token.
func logfCtx(ctx context.Context, format string, args ...any) {
	rid, tid := correlationFrom(ctx)
	if rid != "" {
		format += " request_id=%s"
		args = append(args, rid)
	}
	if tid != "" {
		format += " trace_id=%s"
		args = append(args, tid)
	}
	log.Printf(format, args...)
}
