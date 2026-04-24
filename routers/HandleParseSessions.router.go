package routers

import (
	"net/http"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/structs"
)

func HandleListSessions(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		responder.SendError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	sessions, err := query.ListSessions(db.DB, query.ListSessionsRequest{
		Select: &query.SelectSession{
			UserID: int64(userID),
		},
	})
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "Failed to fetch sessions", err)
		return
	}

	public := make([]structs.PublicSession, len(sessions))
	for i, s := range sessions {
		public[i] = s.ToPublic()
	}

	responder.New(w, public, "Sessions retrieved successfully")
}
