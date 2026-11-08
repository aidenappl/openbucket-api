package aws

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
)

type UploadRequest struct {
	Bucket    string
	Key       string
	Body      io.ReadSeeker
	Overwrite *bool // Optional: to control whether to overwrite existing objects
	// ContentType is stored on the object and served back as its Content-Type.
	// Empty leaves it to the storage backend (binary/octet-stream).
	ContentType string
}

func Upload(ctx context.Context, req UploadRequest) error {
	s3Client, err := GetS3Client(ctx)
	if err != nil {
		return fmt.Errorf("failed to get S3 client: %w", err)
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(req.Bucket),
		Key:    aws.String(req.Key),
		Body:   req.Body,
	}
	if req.ContentType != "" {
		input.ContentType = aws.String(req.ContentType)
	}

	request, _ := s3Client.PutObjectRequest(input)

	if req.Overwrite != nil && !*req.Overwrite {
		request.HTTPRequest.Header.Set("If-None-Match", "*")
	}

	err = request.Send()

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			fmt.Println("AWS Error Code:", aerr.Code())
			fmt.Println("AWS Error Message:", aerr.Message())
		}
		return fmt.Errorf("failed to upload file: %w", err)
	}

	return nil
}
