package routers

import (
	"encoding/json"
	"net/http"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/gorilla/mux"
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

	req.Bucket = mux.Vars(r)["bucket"] // From the core URL

	if req.Bucket == "" || req.Folder == "" || req.Name == "" {
		responder.ErrMissingParam(w, "bucket, folder or name")
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
