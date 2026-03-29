package routers

import (
	"net/http"

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

	req.Bucket = middleware.GetSession(r.Context()).BucketName // From session context

	if req.Bucket == "" {
		responder.ErrMissingParam(w, "bucket")
		return
	}

	req.Folder = r.URL.Query().Get("folder") // From the URL parameters
	if req.Folder == "" {
		responder.ErrMissingParam(w, "folder")
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
