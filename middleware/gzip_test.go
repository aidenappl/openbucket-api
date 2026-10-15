package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGzipAndServerTiming(t *testing.T) {
	json := strings.Repeat(`{"key":"hotels/1/"},`, 200)
	tests := []struct {
		name, accept, contentType, body string
		wantGzip                        bool
	}{
		{"json is compressed", "gzip, br", "application/json", json, true},
		{"downloads pass through", "gzip", "image/jpeg", "\xff\xd8binary", false},
		{"no gzip without Accept-Encoding", "", "application/json", json, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := LoggingMiddleware(GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(http.StatusTeapot)
				io.WriteString(w, tt.body)
			})))
			req := httptest.NewRequest(http.MethodGet, "/core/v1/1/contents", nil)
			if tt.accept != "" {
				req.Header.Set("Accept-Encoding", tt.accept)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != http.StatusTeapot {
				t.Fatalf("status %d, want 418", rr.Code)
			}
			if st := rr.Header().Get("Server-Timing"); !strings.HasPrefix(st, "total;dur=") {
				t.Fatalf("Server-Timing = %q", st)
			}
			gotGzip := rr.Header().Get("Content-Encoding") == "gzip"
			if gotGzip != tt.wantGzip {
				t.Fatalf("gzip = %v, want %v", gotGzip, tt.wantGzip)
			}
			body := rr.Body.String()
			if gotGzip {
				if rr.Body.Len() >= len(tt.body) {
					t.Fatalf("compressed %d bytes into %d", len(tt.body), rr.Body.Len())
				}
				zr, err := gzip.NewReader(rr.Body)
				if err != nil {
					t.Fatal(err)
				}
				b, _ := io.ReadAll(zr)
				body = string(b)
			}
			if body != tt.body {
				t.Fatalf("body mismatch: got %d bytes, want %d", len(body), len(tt.body))
			}
		})
	}
}
