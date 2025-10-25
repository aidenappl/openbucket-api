package aws

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func PresignObject(ctx context.Context, bucket, key string, expiration int64) (string, error) {
	s3Client, err := GetS3Client(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get S3 client: %w", err)
	}

	if expiration <= 0 {
		expiration = 3600 // Default to 1 hour if not specified or invalid
	}

	req, _ := s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	// Generate a presigned URL valid for the specified expiration time
	urlStr, err := req.Presign(time.Duration(expiration) * time.Second)
	if err != nil {
		log.Fatalf("Failed to sign request: %v", err)
	}

	return urlStr, nil
}
