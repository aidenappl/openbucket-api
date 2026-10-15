package aws

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/aidenappl/openbucket-api/timing"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

// Contents is one level of a bucket: the sub-folders (common prefixes) and
// the objects directly under a prefix.
type Contents struct {
	Folders []string     `json:"folders"`
	Objects []*s3.Object `json:"objects"`
}

// ListContents lists one level under prefix — folders and objects — in a
// single delimited listing, MaxListEntries entries (folders + objects) per
// call. next is empty when the level is complete; otherwise pass it back as
// cursor for the following entries.
//
// The explorer used to call ListFolders and ListObjects for every folder it
// opened: two full listings of the same prefix, each a round trip per page to
// the storage server. This is one.
//
// Entries are cut in key order across folders and objects (byte order, as
// S3 lists them), so a page ends at one exact entry and the cursor (that
// entry, opaque) resumes right after it via StartAfter — openbucket-go skips a
// common prefix the cursor ends on, and everything inside it.
//
// prefix is normalised like ListObjects (a trailing "/" is added), and the
// folder's own marker key is left out.
func ListContents(ctx context.Context, bucket, prefix, cursor string) (contents Contents, next string, err error) {
	defer timing.Track(ctx, "storage")()

	after, err := decodeCursor(cursor)
	if err != nil {
		return Contents{}, "", err
	}
	s3Client, err := GetS3Client(ctx)
	if err != nil {
		return Contents{}, "", fmt.Errorf("failed to get S3 client: %w", err)
	}
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	contents = Contents{Folders: []string{}, Objects: []*s3.Object{}}
	count, last := 0, ""
	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	}
	if after != "" {
		input.StartAfter = aws.String(after)
	}

	err = s3Client.ListObjectsV2PagesWithContext(ctx, input, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, e := range mergeEntries(page, prefix) {
			if count == MaxListEntries {
				next = encodeCursor(last)
				return false
			}
			if e.object != nil {
				if e.object.ETag != nil {
					*e.object.ETag = stripQuotes(*e.object.ETag)
				}
				contents.Objects = append(contents.Objects, e.object)
			} else {
				contents.Folders = append(contents.Folders, e.key)
			}
			last = e.key
			count++
		}
		// Exactly full: whether more exist is only known from the page.
		if count == MaxListEntries && !lastPage {
			next = encodeCursor(last)
			return false
		}
		return !lastPage
	})
	if err != nil {
		return Contents{}, "", err
	}
	return contents, next, nil
}

type entry struct {
	key    string
	object *s3.Object // nil for a common prefix
}

// mergeEntries returns a page's common prefixes and objects as one stream in
// key order, without the folder's own marker.
func mergeEntries(page *s3.ListObjectsV2Output, prefix string) []entry {
	out := make([]entry, 0, len(page.CommonPrefixes)+len(page.Contents))
	for _, cp := range page.CommonPrefixes {
		if cp != nil && cp.Prefix != nil {
			out = append(out, entry{key: *cp.Prefix})
		}
	}
	for _, o := range page.Contents {
		if o != nil && o.Key != nil && *o.Key != prefix {
			out = append(out, entry{key: *o.Key, object: o})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

// ErrInvalidCursor is returned for a cursor this API didn't issue.
var ErrInvalidCursor = errors.New("invalid cursor")

func encodeCursor(key string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(key))
}

func decodeCursor(c string) (string, error) {
	if c == "" {
		return "", nil
	}
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil || len(b) == 0 {
		return "", ErrInvalidCursor
	}
	return string(b), nil
}
