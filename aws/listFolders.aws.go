package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func ListFolders(ctx context.Context, bucket string, prefix string) ([]string, error) {
	s3Client, err := GetS3Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get S3 client: %w", err)
	}

	var folders []string
	err = s3Client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"), // key for folders
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, cp := range page.CommonPrefixes {
			folders = append(folders, *cp.Prefix)
		}
		return true
	})
	if err != nil {
		return nil, err
	}

	if len(folders) == 0 {
		return nil, nil
	}
	return folders, nil

}
