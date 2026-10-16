package middleware

import (
	"net/http"
	"regexp"
	"time"
)

// Per-request deadlines replace the server-wide ReadTimeout/WriteTimeout,
// which applied to every request alike: a 15 s read cut off any upload that
// took longer to send, and a 60 s write cut off long downloads and the reply
// to a long upload (openbucket-api#12).
// Variables only so tests can shorten them.
var (
	DefaultReadDeadline  = 30 * time.Second
	DefaultWriteDeadline = 60 * time.Second
	// TransferDeadline bounds one upload or download.
	TransferDeadline = 2 * time.Hour
)

// Deadlines sets the default read and write deadlines on every request. Wrap
// the whole handler with it (not mux.Use), so unmatched routes and CORS
// preflights get them too; the server itself keeps only ReadHeaderTimeout.
func Deadlines(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extend(w, DefaultReadDeadline, DefaultWriteDeadline)
		next.ServeHTTP(w, r)
	})
}

// LongTransfer gives one route TransferDeadline to read its body and write
// its response, instead of the defaults.
func LongTransfer(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		extend(w, TransferDeadline, TransferDeadline)
		next(w, r)
	}
}

func extend(w http.ResponseWriter, read, write time.Duration) {
	rc := http.NewResponseController(w)
	now := time.Now()
	// ErrNotSupported only outside a real server (tests): nothing to extend.
	_ = rc.SetReadDeadline(now.Add(read))
	_ = rc.SetWriteDeadline(now.Add(write))
}

// MaxBodyBytes caps request bodies; uploads have their own cap (see
// MaxBody).
const MaxBodyBytes = 11 << 20

var uploadPath = regexp.MustCompile(`^/core/v1/[0-9]+/object$`)

// MaxBody caps every request body at MaxBodyBytes, except an object upload
// (PUT /core/v1/{id}/object), which HandleUpload bounds by the upload limit.
// The 11 MB cap used to apply to uploads too, so any file over ~10 MB failed
// although the console allows 5 GB.
func MaxBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !(r.Method == http.MethodPut && uploadPath.MatchString(r.URL.Path)) {
			r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}
