package routers

import (
	"net/http"
	"strconv"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/gorilla/mux"
)

func HandleDeleteSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		responder.SendError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	idStr := mux.Vars(r)["id"]
	sessionID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "invalid session ID")
		return
	}

	if err := query.DeleteSession(db.DB, sessionID, int64(userID)); err != nil {
		if err.Error() == "session not found" {
			responder.SendError(w, http.StatusNotFound, "session not found or not owned by you")
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "failed to delete session", err)
		return
	}

	responder.New(w, nil, "session deleted")
}
