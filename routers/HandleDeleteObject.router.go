package routers

import (
	"net/http"
	"strings"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/responder"
)

func HandleDeleteObject(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r.Context())
	if session == nil {
		responder.SendError(w, http.StatusUnauthorized, "session not found")
		return
	}
	bucket := session.BucketName
	key := r.URL.Query().Get("key") // Object key from query parameters

	if bucket == "" || key == "" {
		responder.ErrMissingParam(w, "bucket or key")
		return
	}

	if strings.Contains(key, "..") {
		responder.SendError(w, http.StatusBadRequest, "invalid key: path traversal not allowed", nil)
		return
	}

	err := aws.DeleteObject(r.Context(), bucket, key)
	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to delete object", err)
		return
	}

	responder.New(w, nil, "Object deleted successfully")
}
