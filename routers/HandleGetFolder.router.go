package routers

import (
	"net/http"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/gorilla/mux"
)

type HandleGetFolderRequest struct {
	Bucket string `json:"bucket"`
	Folder string `json:"folder"`
}

func HandleGetFolder(w http.ResponseWriter, r *http.Request) {
	var req HandleGetFolderRequest
	// Parse variables
	req.Bucket = mux.Vars(r)["bucket"] // From the core URL
	req.Folder = r.URL.Query().Get("folder")
	if req.Bucket == "" || req.Folder == "" {
		responder.ErrMissingParam(w, "bucket or folder")
		return
	}
	// Call the AWS function to get the folder
	folder, err := aws.GetFolder(r.Context(), req.Bucket, req.Folder)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "Failed to get folder", err)
		return
	}

	// Respond with the folder details
	responder.New(w, folder, "Folder retrieved successfully")

}
