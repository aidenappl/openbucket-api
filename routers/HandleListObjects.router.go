package routers

import (
	"net/http"
	"strings"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/responder"
)

type HandleListObjectsRequest struct {
	Bucket string `json:"bucket"`
	Prefix string `json:"prefix,omitempty"` // Optional prefix to filter objects
}

func HandleListObjects(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r.Context())
	if session == nil {
		responder.SendError(w, http.StatusUnauthorized, "session not found")
		return
	}

	var req HandleListObjectsRequest

	req.Bucket = session.BucketName
	if req.Bucket == "" {
		responder.ErrMissingParam(w, "bucket")
		return
	}

	req.Prefix = r.URL.Query().Get("prefix") // Optional prefix from query parameters
	if req.Prefix != "" {
		// Normalize prefix: if it's not empty and doesn't end with '/', append '/'
		if !strings.HasSuffix(req.Prefix, "/") {
			req.Prefix += "/"
		}

	}

	// Call the AWS function to list objects
	objects, err := aws.ListObjects(r.Context(), req.Bucket, req.Prefix)
	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to list objects", err)
		return
	}

	// Respond with the list of objects
	responder.New(w, objects, "Objects listed successfully")
}
