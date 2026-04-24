package middleware

import (
	"context"
	"log"
	"net/http"
	"strconv"

	"github.com/aidenappl/openbucket-api/cache"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/tools"
	"github.com/gorilla/mux"
)

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

// SessionMiddleware resolves the {sessionId} path parameter, validates ownership,
// decrypts credentials, and injects SessionClaims into context.
func SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, hasUser := GetUserID(r.Context())
		if !hasUser {
			responder.SendError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}

		sessionIDStr := mux.Vars(r)["sessionId"]
		sessionID, err := strconv.ParseInt(sessionIDStr, 10, 64)
		if err != nil || sessionID == 0 {
			responder.SendError(w, http.StatusBadRequest, "invalid session ID")
			return
		}

		// Check session cache first
		sess, cached := cache.GetSession(sessionID)
		if !cached {
			sess, err = query.GetSessionByID(db.DB, sessionID)
			if err != nil {
				responder.SendError(w, http.StatusNotFound, "session not found")
				return
			}
			cache.SetSession(sessionID, sess)
		}

		if sess.UserID != int64(userID) {
			responder.SendError(w, http.StatusForbidden, "session does not belong to authenticated user")
			return
		}

		claims := &tools.SessionClaims{
			SessionID:  sessionID,
			UserID:     sess.UserID,
			BucketName: sess.BucketName,
			Nickname:   sess.Nickname,
			Region:     sess.Region,
			Endpoint:   sess.Endpoint,
		}
		if sess.AccessKey != nil {
			dec, err := tools.Decrypt(*sess.AccessKey)
			if err != nil {
				log.Printf("failed to decrypt access key for session %d: %v", sessionID, err)
				responder.SendError(w, http.StatusInternalServerError, "failed to decrypt session credentials")
				return
			}
			claims.AccessKey = &dec
		}
		if sess.SecretKey != nil {
			dec, err := tools.Decrypt(*sess.SecretKey)
			if err != nil {
				log.Printf("failed to decrypt secret key for session %d: %v", sessionID, err)
				responder.SendError(w, http.StatusInternalServerError, "failed to decrypt session credentials")
				return
			}
			claims.SecretKey = &dec
		}

		ctx := context.WithValue(r.Context(), SessionContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
