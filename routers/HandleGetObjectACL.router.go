package routers

import (
	"net/http"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/gorilla/mux"
)

func HandleGetObjectACL(w http.ResponseWriter, r *http.Request) {
	//  Get mux variables
	vars := mux.Vars(r)
	bucket := vars["bucket"]

	key := r.URL.Query().Get("key") // Object key from query parameters

	if bucket == "" || key == "" {
		responder.ErrMissingParam(w, "bucket or key")
		return
	}

	res, err := aws.GetObjectACL(r.Context(), bucket, key)
	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to get object ACL", err)
		return
	}

	responder.New(w, res, "Object retrieved successfully")
}
