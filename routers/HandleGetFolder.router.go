package routers

import (
	"net/http"
	"strings"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/responder"
)

type HandleGetFolderRequest struct {
	Bucket string `json:"bucket"`
	Folder string `json:"folder"`
}

func HandleGetFolder(w http.ResponseWriter, r *http.Request) {
	var req HandleGetFolderRequest
	// Parse variables
	session := middleware.GetSession(r.Context())
	if session == nil {
		responder.SendError(w, http.StatusUnauthorized, "session not found")
		return
	}
	req.Bucket = session.BucketName
	req.Folder = r.URL.Query().Get("folder")
	if req.Bucket == "" || req.Folder == "" {
		responder.ErrMissingParam(w, "bucket or folder")
		return
	}
	if strings.Contains(req.Folder, "..") {
		responder.SendError(w, http.StatusBadRequest, "invalid folder: path traversal not allowed", nil)
		return
	}
	// Call the AWS function to get the folder
	folder, err := aws.GetFolder(r.Context(), req.Bucket, req.Folder)
	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to get folder", err)
		return
	}

	// Respond with the folder details
	responder.New(w, folder, "Folder retrieved successfully")

}
