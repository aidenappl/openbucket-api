package routers

import (
	"net/http"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/gorilla/mux"
)

type HandleRenameObjectRequest struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
	NewKey string `json:"newKey"`
}

func HandleRenameObject(w http.ResponseWriter, r *http.Request) {
	var req HandleRenameObjectRequest

	req.Bucket = mux.Vars(r)["bucket"]
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

	err := aws.RenameObject(r.Context(), req.Bucket, req.Key, req.NewKey)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "Failed to rename object", err)
		return
	}

	responder.New(w, err, "Object renamed successfully")
}
