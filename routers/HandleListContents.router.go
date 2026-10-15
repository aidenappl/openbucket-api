package routers

import (
	"net/http"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/responder"
)

// HandleListContents serves GET /contents?prefix= — one folder level, its
// sub-folders and objects, from a single listing (see aws.ListContents).
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

	contents, truncated, err := aws.ListContents(r.Context(), session.BucketName, r.URL.Query().Get("prefix"))
	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to list folder contents", err)
		return
	}

	responder.NewWithPagination(w, contents, responder.Truncation{Truncated: truncated, Limit: aws.MaxListEntries}, "Folder contents listed successfully")
}
