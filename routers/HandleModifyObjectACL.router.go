package routers

import (
	"encoding/json"
	"net/http"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/gorilla/mux"
)

type ObjectACL string

const (
	ACLPrivate    ObjectACL = "private"
	ACLPublicRead ObjectACL = "public-read"
)

type HandleModifyObjectACLRequest struct {
	Bucket string    `json:"bucket"`
	Key    string    `json:"key"`
	ACL    ObjectACL `json:"acl"`
}

func HandleModifyObjectACL(w http.ResponseWriter, r *http.Request) {
	var req HandleModifyObjectACLRequest

	// Parse the request body
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	vars := mux.Vars(r)
	req.Bucket = vars["bucket"]

	req.Key = r.URL.Query().Get("key")

	if req.Bucket == "" || req.Key == "" || req.ACL == "" {
		responder.ErrMissingParam(w, "bucket, key, or acl")
		return
	}

	// validate ACL
	switch req.ACL {
	case ACLPrivate, ACLPublicRead:
		// valid
	default:
		responder.SendError(w, http.StatusBadRequest, "Invalid ACL value", nil)
		return
	}

	// Call the AWS function to modify object ACL
	err = aws.ModifyObjectACL(r.Context(), req.Bucket, req.Key, string(req.ACL))
	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to modify object ACL", err)
		return
	}

	responder.New(w, nil, "Object ACL modified successfully")
}
