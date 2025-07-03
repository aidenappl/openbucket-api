package routers

import (
	"net/http"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/responder"
)

type HandleUploadRequest struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
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

	req.Bucket = r.FormValue("bucket")
	if req.Bucket == "" {
		responder.ErrMissingParam(w, "bucket")
		return
	}

	req.Key = r.FormValue("key")
	if req.Key == "" {
		req.Key = header.Filename // Use the original filename if no key is provided
	}

	err = aws.Upload(aws.UploadRequest{
		Bucket: req.Bucket,
		Key:    req.Key,
		Body:   file,
	})
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "Failed to upload file", err)
		return
	}

	responder.New(w, nil, "File uploaded successfully")
}
