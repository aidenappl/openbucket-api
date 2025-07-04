package routers

import (
	"net/http"
	"strconv"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/gorilla/mux"
)

type HandlePresignRequest struct {
	Bucket     string `json:"bucket"`
	Key        string `json:"key"`
	Expiration int64  `json:"expiration,omitempty"` // Optional expiration time in seconds
	// Additional fields like Expiry can be added here
}

func HandlePresign(w http.ResponseWriter, r *http.Request) {
	var req HandlePresignRequest

	req.Bucket = mux.Vars(r)["bucket"]
	if req.Bucket == "" {
		responder.ErrMissingParam(w, "bucket")
		return
	}

	req.Key = r.URL.Query().Get("key")
	if req.Key == "" {
		responder.ErrMissingParam(w, "key")
		return
	}

	// Optional expiration time in seconds
	expiration := r.URL.Query().Get("expiration")
	if expiration != "" {
		var err error
		intExp, err := strconv.Atoi(expiration)
		if err != nil || intExp <= 0 {
			responder.SendError(w, http.StatusBadRequest, "Invalid expiration time", err)
			return
		}
		req.Expiration = int64(intExp)
	} else {
		req.Expiration = 3600 // Default to 1 hour if not specified (seconds)
	}

	// Handle the presigned URL generation logic here
	url, err := aws.PresignObject(req.Bucket, req.Key, req.Expiration)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "Failed to generate presigned URL", err)
		return
	}

	responder.New(w, map[string]string{"url": url, "expiration": strconv.FormatInt(req.Expiration, 10)}, "Presigned URL generated successfully")

}
