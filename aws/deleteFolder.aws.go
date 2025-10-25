package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

type FolderRequest struct {
	Bucket string `json:"bucket"`
	Prefix string `json:"prefix"`
}

func DeleteFolder(ctx context.Context, req FolderRequest) error {

	if !strings.HasSuffix(req.Prefix, "/") {
		req.Prefix += "/"
	}

	s3Client, err := GetS3Client(ctx)
	if err != nil {
		return fmt.Errorf("failed to get S3 client: %w", err)
	}

	// List all objects under this prefix
	var toDelete []*s3.ObjectIdentifier
	err = s3Client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(req.Bucket),
		Prefix: aws.String(req.Prefix),
	}, func(page *s3.ListObjectsV2Output, last bool) bool {
		for _, obj := range page.Contents {
			toDelete = append(toDelete, &s3.ObjectIdentifier{Key: obj.Key})
		}
		return !last
	})
	if err != nil {
		return err
	}

	if len(toDelete) == 0 {
		return nil // Nothing to delete
	}

	// Delete in chunks of 1000
	for i := 0; i < len(toDelete); i += 1000 {
		end := i + 1000
		if end > len(toDelete) {
			end = len(toDelete)
		}
		_, err = s3Client.DeleteObjects(&s3.DeleteObjectsInput{
			Bucket: aws.String(req.Bucket),
			Delete: &s3.Delete{
				Objects: toDelete[i:end],
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}
