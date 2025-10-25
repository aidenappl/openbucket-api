package routers

import (
	"encoding/json"
	"net/http"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/gorilla/mux"
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

	req.Bucket = mux.Vars(r)["bucket"] // From the core URL

	if req.Bucket == "" || req.Folder == "" {
		responder.ErrMissingParam(w, "bucket or folder")
		return
	}

	// Call the AWS function to create a folder
	err = aws.CreateFolder(r.Context(), req.Bucket, req.Folder)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "Failed to create folder", err)
		return
	}

	responder.New(w, nil, "Folder created successfully")
}
