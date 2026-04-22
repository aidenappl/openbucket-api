package routers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/responder"
)

type HandleUpdateFolderRequest struct {
	Bucket string `json:"bucket"`
	Folder string `json:"folder"`
	Name   string `json:"name"` // New name for the folder
}

func HandleUpdateFolder(w http.ResponseWriter, r *http.Request) {
	var req HandleUpdateFolderRequest

	// Parse the request body
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	session := middleware.GetSession(r.Context())
	if session == nil {
		responder.SendError(w, http.StatusUnauthorized, "session not found")
		return
	}
	req.Bucket = session.BucketName

	if req.Bucket == "" || req.Folder == "" || req.Name == "" {
		responder.ErrMissingParam(w, "bucket, folder or name")
		return
	}
	if strings.Contains(req.Folder, "..") || strings.Contains(req.Name, "..") {
		responder.SendError(w, http.StatusBadRequest, "invalid folder: path traversal not allowed", nil)
		return
	}

	if req.Name != req.Folder {
		err := aws.RenameFolder(r.Context(), req.Bucket, req.Folder, req.Name)
		if err != nil {
			if aws.CheckError(err, w, r) {
				return
			}
			responder.SendError(w, http.StatusInternalServerError, "Failed to rename folder", err)
			return
		}
	}

	responder.New(w, nil, "Folder updated successfully")
}
