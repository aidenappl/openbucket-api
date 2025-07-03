package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func GetObject(bucket, key string) (*s3.GetObjectOutput, error) {
	// Create a new request to get the object from S3
	req := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	// Call the S3 GetObject function
	result, err := s3Client.GetObject(req)
	if err != nil {
		return nil, err
	}

	// Check if the object was found
	if result == nil {
		return nil, fmt.Errorf("object not found: bucket=%s, key=%s", bucket, key)
	}

	return result, nil
}
