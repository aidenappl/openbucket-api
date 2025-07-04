package aws

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func ListObjects(bucket, prefix string) ([]string, error) {
	var keys []string

	// Normalize prefix: if it's not empty and doesn't end with '/', append '/'
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix), // Filter to this "directory"
		Delimiter: aws.String("/"),    // Only list items directly in the folder, not recursively
	}

	err := s3Client.ListObjectsV2Pages(input, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, item := range page.Contents {
			if item != nil && item.Key != nil {
				// Exclude the folder key itself if present
				if *item.Key != prefix {
					keys = append(keys, *item.Key)
				}
			}
		}
		return !lastPage
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	return keys, nil
}
