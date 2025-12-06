package routers

import (
	"net/http"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/gorilla/mux"
)

func HandleListFolders(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	vars := r.URL.Query()
	bucket := mux.Vars(r)["bucket"] // From the core URL
	prefix := vars.Get("prefix")
	if bucket == "" {
		responder.ErrMissingParam(w, "bucket")
		return
	}
	if prefix == "" {
		prefix = "" // Default to empty prefix if not provided
	}

	// Call the AWS function to list folders
	folders, err := aws.ListFolders(r.Context(), bucket, prefix)
	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to list folders", err)
		return
	}

	// Respond with the list of folders
	responder.New(w, folders, "Folders listed successfully")
}
