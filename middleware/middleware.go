package middleware

import (
	"context"
	"log"
	"net/http"

	"github.com/aidenappl/openbucket-api/tools"
)

type contextKey string

const SessionContextKey contextKey = "session"

// LoggingMiddleware logs the request method and URI
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

// Token Handling
func TokenAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ignore session routes
		if r.URL.Path == "/core/v1/sessions" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		} else {
			if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			tokenString := authHeader[7:]
			token, err := tools.DecodeAndDecryptSession(tokenString)
			if err != nil || token == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), SessionContextKey, token)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// MuxHeaderMiddleware sets the headers for the response
func MuxHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, "+
			"Content-Type, "+
			"Accept-Encoding, "+
			"Connection, "+
			"Content-Length")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Content-Type", "application/json")
		w.Header().Add("Server", "Go")
		next.ServeHTTP(w, r)
	})
}
