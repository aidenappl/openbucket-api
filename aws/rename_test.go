package aws

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
)

// memS3 is an in-memory bucket "b" answering HEAD, ListObjectsV2 (prefix,
// max-keys), CopyObject (PUT + x-amz-copy-source, overwrites like S3) and
// DELETE.
type memS3 struct {
	mu   sync.Mutex
	objs map[string]string
}

func newMemS3(t *testing.T, keys map[string]string) (*memS3, *httptest.Server) {
	m := &memS3{objs: keys}
	srv := httptest.NewServer(http.HandlerFunc(m.serve))
	t.Cleanup(srv.Close)
	return m, srv
}

func (m *memS3) serve(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.TrimPrefix(r.URL.Path, "/b")
	key = strings.TrimPrefix(key, "/")
	switch {
	case r.Method == http.MethodGet && key == "":
		prefix := r.URL.Query().Get("prefix")
		var ks []string
		for k := range m.objs {
			if strings.HasPrefix(k, prefix) {
				ks = append(ks, k)
			}
		}
		sort.Strings(ks)
		if mk := r.URL.Query().Get("max-keys"); mk == "1" && len(ks) > 1 {
			ks = ks[:1]
		}
		var b strings.Builder
		for _, k := range ks {
			fmt.Fprintf(&b, "<Contents><Key>%s</Key><ETag>\"e\"</ETag><Size>%d</Size></Contents>", k, len(m.objs[k]))
		}
		fmt.Fprintf(w, "<ListBucketResult><IsTruncated>false</IsTruncated>%s</ListBucketResult>", b.String())
	case r.Method == http.MethodHead:
		if _, ok := m.objs[key]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("ETag", `"e"`)
	case r.Method == http.MethodPut && r.Header.Get("X-Amz-Copy-Source") != "":
		src, _ := url.PathUnescape(r.Header.Get("X-Amz-Copy-Source"))
		src = strings.TrimPrefix(strings.TrimPrefix(src, "/"), "b/")
		v, ok := m.objs[src]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		m.objs[key] = v
		fmt.Fprint(w, `<CopyObjectResult><ETag>"e"</ETag></CopyObjectResult>`)
	case r.Method == http.MethodDelete:
		delete(m.objs, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (m *memS3) get(k string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.objs[k]
	return v, ok
}

func TestRenameObjectRefusesExistingUnlessOverwrite(t *testing.T) {
	m, srv := newMemS3(t, map[string]string{"a.txt": "A", "b.txt": "B"})
	ctx := sessionContext(srv.URL)

	if err := RenameObject(ctx, "b", "a.txt", "b.txt", false); !errors.Is(err, ErrDestinationExists) {
		t.Fatalf("err = %v, want ErrDestinationExists", err)
	}
	if v, _ := m.get("b.txt"); v != "B" {
		t.Fatal("refused rename must leave the destination alone")
	}
	if err := RenameObject(ctx, "b", "a.txt", "b.txt", true); err != nil {
		t.Fatal(err)
	}
	if v, _ := m.get("b.txt"); v != "A" {
		t.Fatalf("overwrite: b.txt = %q, want A", v)
	}
	if _, ok := m.get("a.txt"); ok {
		t.Fatal("source should be gone after the move")
	}
}

func TestRenameObjectToFreeNameAndToItself(t *testing.T) {
	m, srv := newMemS3(t, map[string]string{"a.txt": "A"})
	ctx := sessionContext(srv.URL)
	if err := RenameObject(ctx, "b", "a.txt", "a.txt", false); err != nil {
		t.Fatalf("self-rename should be a no-op, got %v", err)
	}
	if v, _ := m.get("a.txt"); v != "A" {
		t.Fatal("self-rename lost the object")
	}
	if err := RenameObject(ctx, "b", "a.txt", "c.txt", false); err != nil {
		t.Fatal(err)
	}
	if v, ok := m.get("c.txt"); !ok || v != "A" {
		t.Fatal("rename to a free name failed")
	}
}

func TestRenameFolderGuards(t *testing.T) {
	m, srv := newMemS3(t, map[string]string{"pics/": "", "pics/a.jpg": "A", "old/": "", "old/x.jpg": "X"})
	ctx := sessionContext(srv.URL)

	if err := RenameFolder(ctx, "b", "pics/", "old/"); !errors.Is(err, ErrDestinationExists) {
		t.Fatalf("into existing folder: err = %v, want ErrDestinationExists", err)
	}
	if err := RenameFolder(ctx, "b", "pics/", "pics/2026/"); !errors.Is(err, ErrFolderIntoItself) {
		t.Fatalf("into itself: err = %v, want ErrFolderIntoItself", err)
	}
	if err := RenameFolder(ctx, "b", "pics/", "photos/"); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"photos/", "photos/a.jpg"} {
		if _, ok := m.get(k); !ok {
			t.Fatalf("%s missing after folder rename", k)
		}
	}
	for _, k := range []string{"pics/", "pics/a.jpg"} {
		if _, ok := m.get(k); ok {
			t.Fatalf("%s left behind", k)
		}
	}
}
