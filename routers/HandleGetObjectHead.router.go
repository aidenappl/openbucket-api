package routers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aws/aws-sdk-go/service/s3"
)

func HandleGetObjectHead(w http.ResponseWriter, r *http.Request) {
	//  Get mux variables
	bucket := middleware.GetSession(r.Context()).BucketName

	q := r.URL.Query()
	if _, ok := q["bulk"]; ok {
		handleBulkObjectHead(bucket, w, r)
		return
	}

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

type handleBulkObjectHeadRequest struct {
	Keys []string `json:"keys"`
}

func handleBulkObjectHead(bucket string, w http.ResponseWriter, r *http.Request) {
	var req handleBulkObjectHeadRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "Invalid JSON body", err)
		return
	}

	if bucket == "" || len(req.Keys) == 0 {
		responder.ErrMissingParam(w, "bucket or keys")
		return
	}

	// Pre-allocate results slice to preserve order
	results := make([]*s3.HeadObjectOutput, len(req.Keys))
	var wg sync.WaitGroup

	// Use the request context for all goroutines
	ctx := r.Context()

	for i, key := range req.Keys {
		wg.Add(1)
		go func(idx int, k string, ctx context.Context) {
			defer wg.Done()
			head, err := aws.GetObjectHead(ctx, bucket, k)
			if err != nil {
				// Skip keys that are inaccessible or missing — leave as nil
				return
			}
			results[idx] = head
		}(i, key, ctx)
	}

	wg.Wait()

	responder.New(w, results, "Bulk object heads retrieved successfully")
}
