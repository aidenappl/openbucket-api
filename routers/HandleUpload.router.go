package routers

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/responder"
)

func containsPathTraversal(keys ...string) bool {
	for _, k := range keys {
		if strings.Contains(k, "..") {
			return true
		}
	}
	return false
}

type HandleUploadRequest struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
	Prefix string `json:"prefix,omitempty"` // Optional prefix to filter objects
}

func HandleUpload(w http.ResponseWriter, r *http.Request) {
	req := &HandleUploadRequest{}

	// Parse the request body
	err := r.ParseMultipartForm(10 << 20) // 10MB
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "Failed to parse form data", err)
		return
	}

	// Retrieve the file
	file, header, err := r.FormFile("file") // "file" is the form field name
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "Failed to get file", err)
		return
	}
	defer file.Close()

	buf, err := io.ReadAll(file)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "Failed to read file", err)
		return
	}

	req.Bucket = middleware.GetSession(r.Context()).BucketName
	if req.Bucket == "" {
		responder.ErrMissingParam(w, "bucket")
		return
	}

	req.Prefix = r.FormValue("prefix") // Optional prefix from form data
	if req.Prefix != "" {
		// Normalize prefix: if it's not empty and doesn't end with '/', append '/'
		if !strings.HasSuffix(req.Prefix, "/") {
			req.Prefix += "/"
		}
	}

	req.Key = r.FormValue("key")
	if req.Key == "" {
		req.Key = header.Filename // Use the original filename if no key is provided
	}

	if containsPathTraversal(req.Key, req.Prefix) {
		responder.SendError(w, http.StatusBadRequest, "invalid key: path traversal not allowed", nil)
		return
	}

	err = aws.Upload(r.Context(), aws.UploadRequest{
		Bucket: req.Bucket,
		Key:    req.Prefix + req.Key,
		Body:   bytes.NewReader(buf),
	})
	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to upload file", err)
		return
	}

	responder.New(w, nil, "File uploaded successfully")
}
