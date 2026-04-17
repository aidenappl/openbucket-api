package routers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/responder"
)

type HandleCreateFolderRequest struct {
	Bucket string `json:"bucket"`
	Folder string `json:"folder"`
}

func HandleCreateFolder(w http.ResponseWriter, r *http.Request) {
	// Parse mux variables
	var req HandleCreateFolderRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	req.Bucket = middleware.GetSession(r.Context()).BucketName // From session context

	if req.Bucket == "" || req.Folder == "" {
		responder.ErrMissingParam(w, "bucket or folder")
		return
	}

	if strings.Contains(req.Folder, "..") {
		responder.SendError(w, http.StatusBadRequest, "invalid key: path traversal not allowed", nil)
		return
	}

	// Call the AWS function to create a folder
	err = aws.CreateFolder(r.Context(), req.Bucket, req.Folder)
	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to create folder", err)
		return
	}

	responder.New(w, nil, "Folder created successfully")
}
