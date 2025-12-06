package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func GetObjectHead(ctx context.Context, bucket, key string) (*s3.HeadObjectOutput, error) {
	s3Client, err := GetS3Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get S3 client: %w", err)
	}

	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	result, err := s3Client.HeadObject(input)
	if err != nil {
		return nil, err
	}

	// Check if the object was found
	if result == nil {
		return nil, fmt.Errorf("object not found: bucket=%s, key=%s", bucket, key)
	}

	return result, nil
}
