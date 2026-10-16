package routers

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/responder"
)

// MaxUploadBytes caps one upload (file plus multipart overhead). It matches
// openbucket-web's 5 GB limit.
const MaxUploadBytes int64 = 5<<30 + 1<<20

// maxFieldBytes caps the text fields (key, prefix).
const maxFieldBytes = 4 << 10

func containsPathTraversal(keys ...string) bool {
	for _, k := range keys {
		if strings.Contains(k, "..") || strings.HasPrefix(k, "/") || strings.ContainsAny(k, "\x00") {
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

// HandleUpload serves PUT /core/v1/{id}/object: a multipart form with the
// file in `file` and optional `key` and `prefix` fields.
//
// ⚠️ The file is streamed to a temporary file and uploaded from there. It
// used to be read into memory whole (ParseMultipartForm + io.ReadAll), behind
// an 11 MB body cap and a 15 s server read timeout, so the console's "5 GB"
// uploads failed above ~10 MB and a large one could exhaust the container's
// memory (openbucket-api#12). The temporary file, not a pipe, because
// aws-sdk-go v1 needs a seekable body to sign the upload.
func HandleUpload(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r.Context())
	if session == nil {
		responder.SendError(w, http.StatusUnauthorized, "session not found")
		return
	}
	req := &HandleUploadRequest{Bucket: session.BucketName}
	if req.Bucket == "" {
		responder.ErrMissingParam(w, "bucket")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "Failed to parse form data", err)
		return
	}

	var (
		tmp      *os.File
		filename string
		size     int64
	)
	defer func() {
		if tmp != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			uploadReadError(w, err)
			return
		}
		switch part.FormName() {
		case "file":
			if tmp != nil {
				responder.SendError(w, http.StatusBadRequest, "only one file per upload", nil)
				return
			}
			tmp, err = os.CreateTemp("", "ob-upload-*")
			if err != nil {
				responder.SendError(w, http.StatusInternalServerError, "Failed to buffer upload", err)
				return
			}
			filename = part.FileName()
			size, err = io.Copy(tmp, part)
			if err != nil {
				uploadReadError(w, err)
				return
			}
		case "key", "prefix":
			v, err := io.ReadAll(io.LimitReader(part, maxFieldBytes+1))
			if err != nil {
				uploadReadError(w, err)
				return
			}
			if len(v) > maxFieldBytes {
				responder.SendError(w, http.StatusBadRequest, part.FormName()+" is too long", nil)
				return
			}
			if part.FormName() == "key" {
				req.Key = string(v)
			} else {
				req.Prefix = string(v)
			}
		}
		part.Close()
	}

	if tmp == nil {
		responder.SendError(w, http.StatusBadRequest, "Failed to get file", errors.New("no file part"))
		return
	}

	// Normalize prefix: if it's not empty and doesn't end with '/', append '/'
	if req.Prefix != "" && !strings.HasSuffix(req.Prefix, "/") {
		req.Prefix += "/"
	}
	if req.Key == "" {
		req.Key = filename // Use the original filename if no key is provided
	}
	if req.Key == "" {
		responder.ErrMissingParam(w, "key")
		return
	}
	if containsPathTraversal(req.Key, req.Prefix) {
		responder.SendError(w, http.StatusBadRequest, "invalid key: path traversal not allowed", nil)
		return
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		responder.SendError(w, http.StatusInternalServerError, "Failed to read buffered upload", err)
		return
	}
	err = aws.Upload(r.Context(), aws.UploadRequest{
		Bucket: req.Bucket,
		Key:    req.Prefix + req.Key,
		Body:   tmp,
	})
	if err != nil {
		if aws.CheckError(err, w, r) {
			return
		}
		responder.SendError(w, http.StatusInternalServerError, "Failed to upload file", err)
		return
	}

	log.Printf("upload: %s (%d bytes)", req.Prefix+req.Key, size)
	responder.New(w, nil, "File uploaded successfully")
}

// uploadReadError reports a failure reading the request body: 413 when it
// exceeded MaxUploadBytes, 400 otherwise (client gone, malformed multipart).
func uploadReadError(w http.ResponseWriter, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		responder.SendError(w, http.StatusRequestEntityTooLarge, "upload exceeds the 5 GB limit", err)
		return
	}
	responder.SendError(w, http.StatusBadRequest, "Failed to read upload", err)
}
