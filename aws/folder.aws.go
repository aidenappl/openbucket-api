package aws

import (
	"bytes"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
)

// CreateFolder safely creates a "folder" (prefix object) in a bucket without overwriting.
func CreateFolder(bucket, folder string) error {
	if folder == "" || folder[len(folder)-1] != '/' {
		folder += "/"
	}

	// Step 1: Check if it already exists
	_, err := s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(folder),
	})
	if err == nil {
		// It already exists
		return fmt.Errorf("folder '%s' already exists", folder)
	}

	// If it's a real error other than "NotFound", propagate it
	if aerr, ok := err.(awserr.Error); !ok || aerr.Code() != "NotFound" {
		return fmt.Errorf("error checking folder existence: %w", err)
	}

	// Step 2: Upload the zero-byte "folder" object
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(folder),
		Body:   bytes.NewReader([]byte{}),
	})
	if err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}

	return nil
}
