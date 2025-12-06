package routers

import (
	"log"
	"net/http"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/gorilla/mux"
)

func HandleGetObjectHead(w http.ResponseWriter, r *http.Request) {
	//  Get mux variables
	vars := mux.Vars(r)
	bucket := vars["bucket"]

	key := r.URL.Query().Get("key") // Object key from query parameters

	if bucket == "" || key == "" {
		log.Println("Missing bucket or key parameter")
		responder.ErrMissingParam(w, "bucket or key")
		return
	}

	res, err := aws.GetObjectHead(r.Context(), bucket, key)
	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to get object head", err)
		return
	}

	responder.New(w, res, "Object retrieved successfully")
}
