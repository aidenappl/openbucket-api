package aws

import (
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
}

func Upload(req UploadRequest) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(req.Bucket),
		Key:    aws.String(req.Key),
		Body:   req.Body,
	}

	request, _ := s3Client.PutObjectRequest(input)

	// Use conditional write to prevent overwriting
	if req.Overwrite == nil || !*req.Overwrite {
		request.HTTPRequest.Header.Set("If-None-Match", "*")
	}

	err := request.Send()

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			fmt.Println("AWS Error Code:", aerr.Code())
			fmt.Println("AWS Error Message:", aerr.Message())
		}
		return fmt.Errorf("failed to upload file: %w", err)
	}

	return nil
}
