package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aidenappl/openbucket-api/timing"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func stripQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// MaxListEntries caps one listing call. A folder holding more is returned
// truncated (the bool), and the API says so in `pagination.truncated`.
const MaxListEntries = 10000

// ListObjects returns the objects directly under prefix (not recursive), up
// to MaxListEntries. truncated reports that more exist.
//
// ⚠️ It used to stop at the first page whenever the storage server returned no
// continuation token — openbucket-go never did (fixed in openbucket-go), so
// folders silently stopped at 1000 entries.
func ListObjects(ctx context.Context, bucket, prefix string) (objects []*s3.Object, truncated bool, err error) {
	defer timing.Track(ctx, "storage")()

	s3Client, err := GetS3Client(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get S3 client: %w", err)
	}

	// Normalize prefix: if it's not empty and doesn't end with '/', append '/'
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix), // Filter to this "directory"
		Delimiter: aws.String("/"),    // Only list items directly in the folder, not recursively
	}

	err = s3Client.ListObjectsV2Pages(input, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, item := range page.Contents {
			if item != nil && item.Key != nil {
				// Exclude the folder key itself if present
				if *item.Key != prefix {
					if item.ETag != nil {
						*item.ETag = stripQuotes(*item.ETag)
					}
					objects = append(objects, item)
				}
			}
		}
		if len(objects) >= MaxListEntries {
			truncated = len(objects) > MaxListEntries || !lastPage
			return false
		}
		return !lastPage
	})

	if err != nil {
		return nil, false, err
	}
	if len(objects) > MaxListEntries {
		objects = objects[:MaxListEntries]
	}
	return objects, truncated, nil
}
