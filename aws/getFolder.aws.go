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

	out, err := s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list objects in bucket %s with prefix %s: %w", bucket, prefix, err)
	}

	var objects []Object
	for _, item := range out.Contents {
		if *item.Key == prefix {
			// skip the placeholder "folder" object itself
			continue
		}
		objects = append(objects, Object{
			Key:          *item.Key,
			Size:         *item.Size,
			LastModified: item.LastModified.String(),
		})
	}

	return objects, nil
}
