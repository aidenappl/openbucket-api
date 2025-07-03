package routers

import (
	"net/http"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/gorilla/mux"
)

type HandleListObjectsRequest struct {
	Bucket string `json:"bucket"`
}

func HandleListObjects(w http.ResponseWriter, r *http.Request) {
	var req HandleListObjectsRequest

	// Parse mux variables
	req.Bucket = mux.Vars(r)["bucket"]
	if req.Bucket == "" {
		responder.ErrMissingParam(w, "bucket")
		return
	}

	// Call the AWS function to list objects
	objects, err := aws.ListObjects(req.Bucket)
	if err != nil {
		http.Error(w, "Failed to list objects: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with the list of objects
	responder.New(w, objects, "Objects listed successfully")
}
