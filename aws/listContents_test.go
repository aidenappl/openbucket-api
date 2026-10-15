package aws

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/structs"
	"github.com/aidenappl/openbucket-api/tools"
)

// fakeStorage serves ListObjectsV2 with a delimiter over keys the way
// openbucket-go does: one key-ordered stream of keys and common prefixes, cut
// into pages of pageSize, token = base64 of the last entry.
func fakeStorage(t *testing.T, keys []string, pageSize int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	sort.Strings(keys)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		q := r.URL.Query()
		prefix, delim := q.Get("prefix"), q.Get("delimiter")
		after := ""
		if tok := q.Get("continuation-token"); tok != "" {
			b, _ := base64.RawURLEncoding.DecodeString(tok)
			after = string(b)
		}
		var body strings.Builder
		emitted, last, lastCP, truncated := 0, "", "", false
		for _, k := range keys {
			if !strings.HasPrefix(k, prefix) {
				continue
			}
			entry, isCP := k, false
			if i := strings.Index(k[len(prefix):], delim); delim != "" && i != -1 {
				entry, isCP = k[:len(prefix)+i+len(delim)], true
			}
			if entry <= after || (isCP && entry == lastCP) {
				continue
			}
			if emitted == pageSize {
				truncated = true
				break
			}
			if isCP {
				fmt.Fprintf(&body, "<CommonPrefixes><Prefix>%s</Prefix></CommonPrefixes>", entry)
				lastCP = entry
			} else {
				fmt.Fprintf(&body, `<Contents><Key>%s</Key><ETag>"e"</ETag><Size>1</Size></Contents>`, k)
			}
			last = entry
			emitted++
		}
		next := ""
		if truncated {
			next = "<NextContinuationToken>" + base64.RawURLEncoding.EncodeToString([]byte(last)) + "</NextContinuationToken>"
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<ListBucketResult><IsTruncated>%t</IsTruncated>%s%s<KeyCount>%d</KeyCount></ListBucketResult>`,
			truncated, next, body.String(), emitted)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

var testSessionID atomic.Int64

func sessionContext(endpoint string) context.Context {
	ak, sk := "k", "s"
	claims := &tools.SessionClaims{
		SessionID: 1_000_000 + testSessionID.Add(1), // the AWS session cache is keyed by this
		UserID:    7, BucketName: "b", Region: "openbucket", Endpoint: endpoint,
		AccessKey: &ak, SecretKey: &sk,
	}
	ctx := context.WithValue(context.Background(), middleware.UserContextKey, &structs.User{ID: 7, Active: true})
	return context.WithValue(ctx, middleware.SessionContextKey, claims)
}

func TestListContents(t *testing.T) {
	keys := []string{"a/", "a/1.txt", "a/2.txt", "a/b/x.txt", "a/b/y.txt", "a/c/z.txt", "a/d.txt", "a-other/q.txt"}
	for n := 0; n < 30; n++ {
		keys = append(keys, "a/f"+strconv.Itoa(n)+"/k.txt")
	}

	tests := []struct {
		name, prefix         string
		pageSize             int
		wantFolders, wantObj int
	}{
		{"one page", "a/", 1000, 32, 3},
		{"many small pages are all followed", "a/", 4, 32, 3},
		{"prefix without slash is normalised", "a", 4, 32, 3},
		{"root", "", 2, 2, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := fakeStorage(t, append([]string(nil), keys...), tt.pageSize)
			got, truncated, err := ListContents(sessionContext(srv.URL), "b", tt.prefix)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Folders) != tt.wantFolders || len(got.Objects) != tt.wantObj || truncated {
				t.Fatalf("got %d folders, %d objects, truncated=%v; want %d, %d, false",
					len(got.Folders), len(got.Objects), truncated, tt.wantFolders, tt.wantObj)
			}
			for _, o := range got.Objects {
				if *o.Key == "a/" {
					t.Fatal("the folder's own marker key must not be listed")
				}
				if *o.ETag != "e" {
					t.Fatalf("ETag quotes not stripped: %q", *o.ETag)
				}
			}
		})
	}
}

func TestListContentsCapsAtMaxListEntries(t *testing.T) {
	var keys []string
	for n := 0; n < MaxListEntries+50; n++ {
		keys = append(keys, fmt.Sprintf("big/%05d.txt", n))
	}
	srv, calls := fakeStorage(t, keys, 1000)
	got, truncated, err := ListContents(sessionContext(srv.URL), "b", "big/")
	if err != nil {
		t.Fatal(err)
	}
	if !truncated || len(got.Objects) != MaxListEntries {
		t.Fatalf("got %d objects, truncated=%v; want %d, true", len(got.Objects), truncated, MaxListEntries)
	}
	if c := calls.Load(); c != int32(MaxListEntries/1000) {
		t.Fatalf("made %d listing calls, want %d (stop at the cap)", c, MaxListEntries/1000)
	}
}
