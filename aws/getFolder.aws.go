package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func GetFolder(ctx context.Context, bucket string, prefix string) ([]Object, error) {
	if bucket == "" || prefix == "" {
		return nil, fmt.Errorf("bucket and prefix are required")
	}

	s3Client, err := GetS3Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get S3 client: %w", err)
	}

	var objects []Object
	err = s3Client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, item := range page.Contents {
			if item == nil || item.Key == nil {
				continue
			}
			if *item.Key == prefix {
				// skip the placeholder "folder" object itself
				continue
			}
			var size int64
			if item.Size != nil {
				size = *item.Size
			}
			var lastModified string
			if item.LastModified != nil {
				lastModified = item.LastModified.String()
			}
			objects = append(objects, Object{
				Key:          *item.Key,
				Size:         size,
				LastModified: lastModified,
			})
		}
		return len(objects) < 10000
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list objects in bucket %s with prefix %s: %w", bucket, prefix, err)
	}

	return objects, nil
}
