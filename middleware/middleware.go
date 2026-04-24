package middleware

import (
	"log"
	"net/http"
)

// LoggingMiddleware logs the request method and URI.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/" || r.RequestURI == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		log.Println(r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}
