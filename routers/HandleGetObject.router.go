package routers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/responder"
)

func HandleGetObject(w http.ResponseWriter, r *http.Request) {
	//  Get mux variables
	bucket := middleware.GetSession(r.Context()).BucketName

	key := r.URL.Query().Get("key") // Object key from query parameters

	if bucket == "" || key == "" {
		log.Println("Missing bucket or key parameter")
		responder.ErrMissingParam(w, "bucket or key")
		return
	}

	if strings.Contains(key, "..") {
		responder.SendError(w, http.StatusBadRequest, "invalid key: path traversal not allowed", nil)
		return
	}

	res, err := aws.GetObject(r.Context(), bucket, key)

	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to get object", err)
		return
	}
	if res.Body != nil {
		defer res.Body.Close()
	}

	// Stream the object body directly to the client
	if res.ContentType != nil {
		w.Header().Set("Content-Type", *res.ContentType)
	}
	if res.ContentLength != nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", *res.ContentLength))
	}
	w.WriteHeader(http.StatusOK)
	io.Copy(w, res.Body)
}
