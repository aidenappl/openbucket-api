package routers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/responder"
)

type HandlePresignRequest struct {
	Bucket     string `json:"bucket"`
	Key        string `json:"key"`
	Expiration int64  `json:"expiration,omitempty"` // Optional expiration time in seconds
	// Additional fields like Expiry can be added here
}

func HandlePresign(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r.Context())
	if session == nil {
		responder.SendError(w, http.StatusUnauthorized, "session not found")
		return
	}

	var req HandlePresignRequest

	req.Bucket = session.BucketName
	if req.Bucket == "" {
		responder.ErrMissingParam(w, "bucket")
		return
	}

	req.Key = r.URL.Query().Get("key")
	if req.Key == "" {
		responder.ErrMissingParam(w, "key")
		return
	}

	if strings.Contains(req.Key, "..") {
		responder.SendError(w, http.StatusBadRequest, "invalid key: path traversal not allowed", nil)
		return
	}

	// Optional expiration time in seconds; see presignExpiration for the rules.
	exp, err := presignExpiration(r.URL.Query().Get("expiration"))
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "Invalid expiration time", err)
		return
	}
	req.Expiration = exp

	// Handle the presigned URL generation logic here
	url, err := aws.PresignObject(r.Context(), req.Bucket, req.Key, req.Expiration)
	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to generate presigned URL", err)
		return
	}

	responder.New(w, map[string]string{"url": url, "expiration": strconv.FormatInt(req.Expiration, 10)}, "Presigned URL generated successfully")

}

// Presign lifetime limits, in seconds.
const (
	defaultPresignSeconds = 3600
	maxPresignSeconds     = 86400
)

// presignExpiration parses the `expiration` query value (integer seconds).
// Empty means the 1-hour default; non-numeric or <= 0 is an error; anything
// above the 24-hour maximum is clamped to it.
//
// ⚠️ Over-max values used to be reset to the 1-hour DEFAULT rather than
// clamped, so a client asking for 7 days silently got a 1-hour link (and
// openbucket-web labelled it as 7 days). The response's `expiration` field
// reports what was actually granted — clients should display that.
func presignExpiration(raw string) (int64, error) {
	if raw == "" {
		return defaultPresignSeconds, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("expiration must be a positive number of seconds, got %q", raw)
	}
	if n > maxPresignSeconds {
		return maxPresignSeconds, nil
	}
	return n, nil
}
