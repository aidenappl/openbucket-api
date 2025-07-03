package aws

import (
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type UploadRequest struct {
	Bucket    string
	Key       string
	Body      io.Reader
	Overwrite *bool // Optional: to control whether to overwrite existing objects
}

func Upload(req UploadRequest) error {
	if req.Overwrite == nil || !*req.Overwrite {
		_, err := s3Client.HeadObject(&s3.HeadObjectInput{
			Bucket: aws.String(req.Bucket),
			Key:    aws.String(req.Key),
		})

		if err == nil {
			return fmt.Errorf("upload aborted: object already exists at key '%s'", req.Key)
		}

		// Only continue if the error is "NoSuchKey" or "NotFound"
		if aerr, ok := err.(awserr.Error); !ok || (aerr.Code() != "NotFound" && aerr.Code() != "NoSuchKey") {
			return fmt.Errorf("failed to check if object exists: %w", err)
		}

	}
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(req.Bucket),
		Key:    aws.String(req.Key),
		Body:   req.Body,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	fmt.Printf("file uploaded to: %s\n", aws.StringValue(&result.Location))
	return nil
}
