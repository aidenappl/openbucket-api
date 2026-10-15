package routers

import (
	"errors"
	"net/http"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/responder"
)

// HandleListContents serves GET /contents?prefix=&cursor= — one folder
// level, its sub-folders and objects, from a single listing (see
// aws.ListContents). A truncated response carries pagination.next_cursor.
func HandleListContents(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r.Context())
	if session == nil {
		responder.SendError(w, http.StatusUnauthorized, "session not found")
		return
	}
	if session.BucketName == "" {
		responder.ErrMissingParam(w, "bucket")
		return
	}

	q := r.URL.Query()
	contents, next, err := aws.ListContents(r.Context(), session.BucketName, q.Get("prefix"), q.Get("cursor"))
	if errors.Is(err, aws.ErrInvalidCursor) {
		responder.SendError(w, http.StatusBadRequest, "invalid cursor", err)
		return
	}
	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to list folder contents", err)
		return
	}

	responder.NewWithPagination(w, contents, responder.Truncation{
		Truncated:  next != "",
		Limit:      aws.MaxListEntries,
		NextCursor: next,
	}, "Folder contents listed successfully")
}
