package routers

import (
	"net/http"
	"strings"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/responder"
)

type HandleRenameObjectRequest struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
	NewKey string `json:"newKey"`
}

func HandleRenameObject(w http.ResponseWriter, r *http.Request) {
	var req HandleRenameObjectRequest

	req.Bucket = middleware.GetSession(r.Context()).BucketName
	if req.Bucket == "" {
		responder.ErrMissingParam(w, "bucket")
		return
	}

	req.Key = r.URL.Query().Get("key")
	if req.Key == "" {
		responder.ErrMissingParam(w, "key")
		return
	}

	req.NewKey = r.URL.Query().Get("newKey")
	if req.NewKey == "" {
		responder.ErrMissingParam(w, "newKey")
		return
	}

	if strings.Contains(req.Key, "..") || strings.Contains(req.NewKey, "..") {
		responder.SendError(w, http.StatusBadRequest, "invalid key: path traversal not allowed", nil)
		return
	}

	err := aws.RenameObject(r.Context(), req.Bucket, req.Key, req.NewKey)
	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to rename object", err)
		return
	}

	responder.New(w, nil, "Object renamed successfully")
}
