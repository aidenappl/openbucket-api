package aws

import (
	"context"
	"errors"
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
		return nil, errors.New("object not found")
	}

	return result, nil
}
