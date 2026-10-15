package aws

import (
	"context"
	"fmt"
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
// single delimited listing, up to MaxListEntries entries in total; truncated
// reports that more exist.
//
// The explorer used to call ListFolders and ListObjects for every folder it
// opened: two full listings of the same prefix, each a round trip per page to
// the storage server through Cloudflare. This is one.
//
// prefix is normalised like ListObjects (a trailing "/" is added), and the
// folder's own marker key is left out.
func ListContents(ctx context.Context, bucket, prefix string) (contents Contents, truncated bool, err error) {
	defer timing.Track(ctx, "storage")()

	s3Client, err := GetS3Client(ctx)
	if err != nil {
		return Contents{}, false, fmt.Errorf("failed to get S3 client: %w", err)
	}
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	contents = Contents{Folders: []string{}, Objects: []*s3.Object{}}
	count := func() int { return len(contents.Folders) + len(contents.Objects) }

	err = s3Client.ListObjectsV2PagesWithContext(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, cp := range page.CommonPrefixes {
			if cp != nil && cp.Prefix != nil {
				contents.Folders = append(contents.Folders, *cp.Prefix)
			}
		}
		for _, item := range page.Contents {
			if item == nil || item.Key == nil || *item.Key == prefix {
				continue
			}
			if item.ETag != nil {
				*item.ETag = stripQuotes(*item.ETag)
			}
			contents.Objects = append(contents.Objects, item)
		}
		if count() >= MaxListEntries {
			truncated = count() > MaxListEntries || !lastPage
			return false
		}
		return !lastPage
	})
	if err != nil {
		return Contents{}, false, err
	}

	// Trim overshoot from the last page: folders keep priority (they are
	// what you navigate by), then objects fill the remainder.
	if count() > MaxListEntries {
		if len(contents.Folders) > MaxListEntries {
			contents.Folders = contents.Folders[:MaxListEntries]
		}
		contents.Objects = contents.Objects[:MaxListEntries-len(contents.Folders)]
	}
	return contents, truncated, nil
}
