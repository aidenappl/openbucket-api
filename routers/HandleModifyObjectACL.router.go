package routers

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/responder"
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
	q := r.URL.Query()
	if _, ok := q["bulk"]; ok {
		handleBulkModifyObjectACL(w, r)
		return
	}

	var req HandleModifyObjectACLRequest

	// Parse the request body
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	req.Bucket = middleware.GetSession(r.Context()).BucketName

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

type handleBulkModifyObjectACLRequest struct {
	Keys []string  `json:"keys"`
	ACL  ObjectACL `json:"acl"`
}

type bulkACLResult struct {
	Key     string `json:"key"`
	Success bool   `json:"success"`
}

func handleBulkModifyObjectACL(w http.ResponseWriter, r *http.Request) {
	bucket := middleware.GetSession(r.Context()).BucketName

	var req handleBulkModifyObjectACLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.SendError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if bucket == "" || len(req.Keys) == 0 || req.ACL == "" {
		responder.ErrMissingParam(w, "bucket, keys, or acl")
		return
	}

	switch req.ACL {
	case ACLPrivate, ACLPublicRead:
		// valid
	default:
		responder.SendError(w, http.StatusBadRequest, "Invalid ACL value", nil)
		return
	}

	results := make([]bulkACLResult, len(req.Keys))
	var wg sync.WaitGroup
	ctx := r.Context()

	for i, key := range req.Keys {
		wg.Add(1)
		go func(idx int, k string) {
			defer wg.Done()
			err := aws.ModifyObjectACL(ctx, bucket, k, string(req.ACL))
			results[idx] = bulkACLResult{Key: k, Success: err == nil}
		}(i, key)
	}

	wg.Wait()

	responder.New(w, results, "Bulk ACL modification complete")
}
