package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func DeleteObject(bucket, key string) error {
	// Create a new request to delete the object from S3
	req := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	// Call the S3 DeleteObject function
	_, err := s3Client.DeleteObject(req)
	if err != nil {
		return err
	}

	return nil
}
