package routers

import (
	"net/http"
	"strings"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/responder"
)

type HandleDeleteFolderRequest struct {
	Bucket string `json:"bucket"`
	Folder string `json:"folder"` // Folder to delete
}

func HandleDeleteFolder(w http.ResponseWriter, r *http.Request) {
	var req HandleDeleteFolderRequest

	session := middleware.GetSession(r.Context())
	if session == nil {
		responder.SendError(w, http.StatusUnauthorized, "session not found")
		return
	}
	req.Bucket = session.BucketName

	if req.Bucket == "" {
		responder.ErrMissingParam(w, "bucket")
		return
	}

	req.Folder = r.URL.Query().Get("folder") // From the URL parameters
	if req.Folder == "" {
		responder.ErrMissingParam(w, "folder")
		return
	}

	if strings.Contains(req.Folder, "..") {
		responder.SendError(w, http.StatusBadRequest, "invalid key: path traversal not allowed", nil)
		return
	}

	err := aws.DeleteFolder(r.Context(), aws.FolderRequest{
		Bucket: req.Bucket,
		Prefix: req.Folder,
	})
	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to delete folder", err)
		return
	}

	responder.New(w, nil, "Folder deleted successfully")
}
