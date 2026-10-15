package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/aidenappl/openbucket-api/timing"
)

// statusResponseWriter captures the status code and adds the Server-Timing
// header just before the headers go out.
type statusResponseWriter struct {
	http.ResponseWriter
	rec         *timing.Recorder
	status      int
	wroteHeader bool
}

func (w *statusResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = code
	w.Header().Set("Server-Timing", w.rec.Header())
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// LoggingMiddleware logs method, URI, status and duration, and gives every
// request a timing.Recorder whose spans (auth, storage, total) are sent as
// Server-Timing — the per-request breakdown shows in the browser's network
// panel. Requests used to be logged with no duration or status, so slow
// endpoints were invisible in the container logs.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, rec := timing.WithRecorder(r.Context())
		sw := &statusResponseWriter{ResponseWriter: w, rec: rec}
		next.ServeHTTP(sw, r.WithContext(ctx))
		if r.RequestURI == "/" || r.RequestURI == "/health" {
			return
		}
		if !sw.wroteHeader {
			sw.status = http.StatusOK
		}
		log.Printf("%s %s %d %s", r.Method, r.RequestURI, sw.status, rec.Elapsed().Round(100*time.Microsecond))
	})
}

// ExposeTiming lets the listed origins read Server-Timing and resource timing
// from scripts (performance.getEntries), which cross-origin needs
// Timing-Allow-Origin for.
func ExposeTiming(origins []string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if o := r.Header.Get("Origin"); allowed[o] {
				w.Header().Set("Timing-Allow-Origin", o)
			}
			next.ServeHTTP(w, r)
		})
	}
}
