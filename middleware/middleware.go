package middleware

import (
	"context"
	"log"
	"net/http"
	"strconv"

	forta "github.com/aidenappl/go-forta"
	"github.com/aidenappl/openbucket-api/cache"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/tools"
	"github.com/gorilla/mux"
)

type contextKey string

const SessionContextKey contextKey = "session"

// LoggingMiddleware logs the request method and URI.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

// GetSession retrieves the SessionClaims injected by SessionMiddleware.
func GetSession(ctx context.Context) *tools.SessionClaims {
	sessionData := ctx.Value(SessionContextKey)
	if sessionData == nil {
		return nil
	}
	session, ok := sessionData.(*tools.SessionClaims)
	if !ok {
		return nil
	}
	return session
}

// FortaMiddleware wraps forta.Protected as a gorilla/mux-compatible middleware
// so that the Forta user ID is in the request context before any downstream
// middleware (e.g. SessionMiddleware) tries to read it.
func FortaMiddleware(next http.Handler) http.Handler {
	return forta.Protected(next.ServeHTTP)
}

// SessionMiddleware resolves the session by the {sessionId} URL variable,
// verifies ownership against the authenticated Forta user ID, decrypts S3
// credentials, and injects a *tools.SessionClaims into the request context.
// Returns 400 for an invalid ID, 404 if not found, 403 if owned by another user.
func SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fortaID, ok := forta.GetFortaIDFromContext(r.Context())
		if !ok {
			http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
			return
		}

		sessionIDStr := mux.Vars(r)["sessionId"]
		sessionID, err := strconv.ParseInt(sessionIDStr, 10, 64)
		if err != nil || sessionID == 0 {
			http.Error(w, `{"error":"invalid session ID"}`, http.StatusBadRequest)
			return
		}

		// Check session cache first
		sess, cached := cache.GetSession(sessionID)
		if !cached {
			var err error
			sess, err = query.GetSessionByID(db.DB, sessionID)
			if err != nil {
				http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
				return
			}
			cache.SetSession(sessionID, sess)
		}

		if sess.FortaUserID != fortaID {
			http.Error(w, `{"error":"session does not belong to authenticated user"}`, http.StatusForbidden)
			return
		}

		claims := &tools.SessionClaims{
			SessionID:   sessionID,
			FortaUserID: sess.FortaUserID,
			BucketName:  sess.BucketName,
			Nickname:    sess.Nickname,
			Region:      sess.Region,
			Endpoint:    sess.Endpoint,
		}
		if sess.AccessKey != nil {
			dec, err := tools.Decrypt(*sess.AccessKey)
			if err == nil {
				claims.AccessKey = &dec
			}
		}
		if sess.SecretKey != nil {
			dec, err := tools.Decrypt(*sess.SecretKey)
			if err == nil {
				claims.SecretKey = &dec
			}
		}

		ctx := context.WithValue(r.Context(), SessionContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
