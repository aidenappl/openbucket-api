package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func ListObjects(bucket string) ([]string, error) {
	var keys []string
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	}

	err := s3Client.ListObjectsV2Pages(input, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, item := range page.Contents {
			if item != nil && item.Key != nil {
				keys = append(keys, *item.Key)
			}
		}
		return !lastPage
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	return keys, nil
}
