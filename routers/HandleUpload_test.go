package routers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"sync"
	"testing"

	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/structs"
	"github.com/aidenappl/openbucket-api/tools"
)

// fakeS3 accepts PutObject and records what it received.
type fakeS3 struct {
	mu    sync.Mutex
	key   string
	sum   [32]byte
	size  int
	ctype string
}

func (f *fakeS3) server(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.key, f.sum, f.size, f.ctype = r.URL.Path, sha256.Sum256(b), len(b), r.Header.Get("Content-Type")
		f.mu.Unlock()
		w.Header().Set("ETag", `"e"`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

var uploadSessionID int64 = 2_000_000

func uploadContext(endpoint string) context.Context {
	uploadSessionID++
	ak, sk := "k", "s"
	claims := &tools.SessionClaims{SessionID: uploadSessionID, UserID: 7, BucketName: "b", Region: "openbucket", Endpoint: endpoint, AccessKey: &ak, SecretKey: &sk}
	ctx := context.WithValue(context.Background(), middleware.UserContextKey, &structs.User{ID: 7, Role: "editor", Active: true})
	return context.WithValue(ctx, middleware.SessionContextKey, claims)
}

// form builds a multipart body; fields are written in the given order.
func form(t *testing.T, parts [][2]string, file []byte) (*bytes.Buffer, string) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, p := range parts {
		if p[0] == "file" {
			fw, _ := mw.CreateFormFile("file", p[1])
			fw.Write(file)
			continue
		}
		mw.WriteField(p[0], p[1])
	}
	mw.Close()
	return &buf, mw.FormDataContentType()
}

func upload(ctx context.Context, body io.Reader, ct string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, "/core/v1/1/object", body).WithContext(ctx)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()
	middleware.MaxBody(http.HandlerFunc(HandleUpload)).ServeHTTP(rr, req)
	return rr
}

func TestHandleUploadStreamsLargeFiles(t *testing.T) {
	file := bytes.Repeat([]byte("0123456789abcdef"), 25<<16) // 25 MB, over the old 11 MB cap
	f := &fakeS3{}
	srv := f.server(t)

	// key and prefix after the file: field order must not matter.
	body, ct := form(t, [][2]string{{"file", "big.bin"}, {"prefix", "a/b"}, {"key", "big.bin"}}, file)
	rr := upload(uploadContext(srv.URL), body, ct)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	if f.key != "/b/a/b/big.bin" || f.size != len(file) || f.sum != sha256.Sum256(file) {
		t.Fatalf("storage got key %q, %d bytes, sum match %v", f.key, f.size, f.sum == sha256.Sum256(file))
	}
}

func TestHandleUploadValidation(t *testing.T) {
	f := &fakeS3{}
	srv := f.server(t)
	tests := []struct {
		name   string
		parts  [][2]string
		status int
	}{
		{"filename used when no key", [][2]string{{"file", "photo.jpg"}}, http.StatusOK},
		{"no file part", [][2]string{{"key", "x"}}, http.StatusBadRequest},
		{"path traversal in key", [][2]string{{"key", "../etc/passwd"}, {"file", "x"}}, http.StatusBadRequest},
		{"leading slash in prefix", [][2]string{{"prefix", "/root"}, {"file", "x"}}, http.StatusBadRequest},
		{"two files", [][2]string{{"file", "a"}, {"file", "b"}}, http.StatusBadRequest},
		{"oversized key field", [][2]string{{"key", strings.Repeat("k", maxFieldBytes+1)}, {"file", "x"}}, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, ct := form(t, tt.parts, []byte("hello"))
			if rr := upload(uploadContext(srv.URL), body, ct); rr.Code != tt.status {
				t.Fatalf("status %d, want %d: %s", rr.Code, tt.status, rr.Body.String())
			}
		})
	}
	if f.key != "/b/photo.jpg" {
		t.Fatalf("filename fallback stored %q", f.key)
	}
}

func TestHandleUploadNotMultipart(t *testing.T) {
	rr := upload(uploadContext("http://127.0.0.1:1"), strings.NewReader("{}"), "application/json")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rr.Code)
	}
}

// APL-105: content_type field > file part header > key extension > backend default.
func TestUploadContentType(t *testing.T) {
	tests := []struct {
		name, field, header, key, want string
	}{
		{"field wins", "text/plain", "image/png", "a.css", "text/plain"},
		{"part header next", "", "image/png", "a.css", "image/png"},
		{"extension next", "", "", "site/app.css", "text/css; charset=utf-8"},
		{"extension case-insensitive", "", "", "LOGO.PNG", "image/png"},
		{"octet-stream header loses to extension", "", "application/octet-stream", "a.js", "text/javascript; charset=utf-8"},
		{"invalid header ignored", "", "not a type", "a.png", "image/png"},
		{"nothing known", "", "", "README", ""},
		{"octet-stream and no extension", "", "application/octet-stream", "blob", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uploadContentType(tt.field, tt.header, tt.key); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandleUploadStoresContentType(t *testing.T) {
	f := &fakeS3{}
	srv := f.server(t)

	// Explicit field.
	body, ct := form(t, [][2]string{{"content_type", "image/svg+xml"}, {"file", "x.bin"}}, []byte("<svg/>"))
	if rr := upload(uploadContext(srv.URL), body, ct); rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	if f.ctype != "image/svg+xml" {
		t.Fatalf("field: stored %q", f.ctype)
	}

	// CreateFormFile sends application/octet-stream; the extension wins.
	body, ct = form(t, [][2]string{{"file", "style.css"}}, []byte("a{}"))
	if rr := upload(uploadContext(srv.URL), body, ct); rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	if f.ctype != "text/css; charset=utf-8" {
		t.Fatalf("extension: stored %q", f.ctype)
	}

	// The file part's own header.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="photo"`)
	h.Set("Content-Type", "image/webp")
	pw, _ := mw.CreatePart(h)
	pw.Write([]byte("RIFF"))
	mw.Close()
	if rr := upload(uploadContext(srv.URL), &buf, mw.FormDataContentType()); rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	if f.ctype != "image/webp" {
		t.Fatalf("part header: stored %q", f.ctype)
	}

	// An unparseable field is a 400.
	body, ct = form(t, [][2]string{{"content_type", "not a type"}, {"file", "a.txt"}}, []byte("x"))
	if rr := upload(uploadContext(srv.URL), body, ct); rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid field: status %d", rr.Code)
	}
}
