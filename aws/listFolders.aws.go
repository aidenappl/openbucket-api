package aws

import (
	"context"
	"fmt"

	"github.com/aidenappl/openbucket-api/timing"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

// ListFolders returns the common prefixes directly under prefix, up to
// MaxListEntries; truncated reports that more exist.
func ListFolders(ctx context.Context, bucket string, prefix string) (folders []string, truncated bool, err error) {
	defer timing.Track(ctx, "storage")()

	s3Client, err := GetS3Client(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get S3 client: %w", err)
	}

	err = s3Client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"), // key for folders
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, cp := range page.CommonPrefixes {
			if cp != nil && cp.Prefix != nil {
				folders = append(folders, *cp.Prefix)
			}
		}
		if len(folders) >= MaxListEntries {
			truncated = len(folders) > MaxListEntries || !lastPage
			return false
		}
		return !lastPage
	})
	if err != nil {
		return nil, false, err
	}

	if len(folders) == 0 {
		return nil, false, nil
	}
	if len(folders) > MaxListEntries {
		folders = folders[:MaxListEntries]
	}
	return folders, truncated, nil

}
