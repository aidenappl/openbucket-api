package routers

import (
	"net/http"

	forta "github.com/aidenappl/go-forta"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/structs"
)

func HandleListSessions(w http.ResponseWriter, r *http.Request) {
	fortaID, ok := forta.GetFortaIDFromContext(r.Context())
	if !ok {
		responder.SendError(w, http.StatusUnauthorized, "unauthenticated", nil)
		return
	}

	sessions, err := query.ListSessions(db.DB, query.ListSessionsRequest{
		Select: &query.SelectSession{
			FortaUserID: fortaID,
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
