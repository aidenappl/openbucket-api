package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTraceIDFrom(t *testing.T) {
	const tid = "4bf92f3577b34da6a3ce929d0e0e4736"
	tests := []struct {
		name        string
		traceparent string
		xTraceID    string
		want        string
	}{
		{"valid traceparent", "00-" + tid + "-00f067aa0ba902b7-01", "", tid},
		{"traceparent wins over X-Trace-ID", "00-" + tid + "-00f067aa0ba902b7-01", "abcdef12", tid},
		{"future version with extra fields", "01-" + tid + "-00f067aa0ba902b7-01-extra", "", tid},
		{"version 00 with extra fields falls back", "00-" + tid + "-00f067aa0ba902b7-01-extra", "abcdef12", "abcdef12"},
		{"version ff falls back", "ff-" + tid + "-00f067aa0ba902b7-01", "abcdef12", "abcdef12"},
		{"all-zero trace id falls back", "00-00000000000000000000000000000000-00f067aa0ba902b7-01", "abcdef12", "abcdef12"},
		{"all-zero parent id falls back", "00-" + tid + "-0000000000000000-01", "abcdef12", "abcdef12"},
		{"uppercase hex is invalid", "00-4BF92F3577B34DA6A3CE929D0E0E4736-00f067aa0ba902b7-01", "abcdef12", "abcdef12"},
		{"malformed traceparent falls back", "garbage", "abcdef12", "abcdef12"},
		{"X-Trace-ID only", "", "abcdef12", "abcdef12"},
		{"nothing", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.traceparent != "" {
				r.Header.Set("traceparent", tt.traceparent)
			}
			if tt.xTraceID != "" {
				r.Header.Set("X-Trace-ID", tt.xTraceID)
			}
			if got := traceIDFrom(r); got != tt.want {
				t.Fatalf("traceIDFrom() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCorrelatedContext(t *testing.T) {
	const uuid = "123e4567-e89b-12d3-a456-426614174000"
	tests := []struct {
		name      string
		requestID string
		traceID   string
		wantRID   string
		wantTID   string
	}{
		{"uuid request id", uuid, "", uuid, ""},
		{"hex ids", "deadbeef", "0123456789abcdef", "deadbeef", "0123456789abcdef"},
		{"invalid request id dropped", "not an id\nforged=1", "deadbeef", "", "deadbeef"},
		{"too short dropped", "abc", "", "", ""},
		{"none", "", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.requestID != "" {
				r.Header.Set("X-Request-ID", tt.requestID)
			}
			if tt.traceID != "" {
				r.Header.Set("X-Trace-ID", tt.traceID)
			}
			rid, tid := correlationFrom(correlatedContext(r))
			if rid != tt.wantRID || tid != tt.wantTID {
				t.Fatalf("correlationFrom() = (%q, %q), want (%q, %q)", rid, tid, tt.wantRID, tt.wantTID)
			}
		})
	}
}
