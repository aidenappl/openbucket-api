package routers

import (
	"net/http"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/gorilla/mux"
)

type HandleDeleteFolderRequest struct {
	Bucket string `json:"bucket"`
	Folder string `json:"folder"` // Folder to delete
}

func HandleDeleteFolder(w http.ResponseWriter, r *http.Request) {
	var req HandleDeleteFolderRequest

	req.Bucket = mux.Vars(r)["bucket"] // From the core URL

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
