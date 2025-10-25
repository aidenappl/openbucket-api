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

	out, err := s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"), // key for folders
	})
	if err != nil {
		return nil, err
	}

	var folders []string
	for _, cp := range out.CommonPrefixes {
		folders = append(folders, *cp.Prefix)
	}

	if len(folders) == 0 {
		return nil, nil
	}
	return folders, nil

}
