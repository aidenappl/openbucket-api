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

// Get Session
func GetSession(ctx context.Context) *tools.SessionClaims {
	sessionData := ctx.Value(SessionContextKey)
	if sessionData == nil {
		log.Println("No session found in context")
		return nil
	}

	session, ok := sessionData.(*tools.SessionClaims)
	if !ok {
		log.Println("Invalid session type in context")
		return nil
	}

	return session
}

// Token Handling
func TokenAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ignore session routes & health checks
		if r.URL.Path == "/core/v1/sessions" || r.URL.Path == "/health" || r.URL.Path == "/" || r.URL.Path == "/core/v1/session" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			log.Println("Missing Authorization header")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		} else {
			if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
				log.Println("Invalid Authorization header format")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			tokenString := authHeader[7:]
			token, err := tools.DecodeAndDecryptSession(tokenString)
			if err != nil || token == nil {
				log.Println("Failed to decode and decrypt session:", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			log.Println("Authenticated request using endpoint:", token.Endpoint)

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
