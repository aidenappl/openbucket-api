package aws

import (
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func PresignObject(bucket, key string, expiration int64) (string, error) {
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
