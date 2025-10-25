package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func RenameObject(ctx context.Context, bucket, sourceKey, destinationKey string) error {
	// Reject renaming folders (by convention, keys ending in '/')
	if strings.HasSuffix(sourceKey, "/") {
		return fmt.Errorf("cannot rename a folder object: %q", sourceKey)
	}

	s3Client, err := GetS3Client(ctx)
	if err != nil {
		return fmt.Errorf("failed to get S3 client: %w", err)
	}

	// Step 1: Copy the object
	_, err = s3Client.CopyObject(&s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(fmt.Sprintf("%s/%s", bucket, sourceKey)),
		Key:        aws.String(destinationKey),
	})
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}

	// Step 2: Confirm the copy succeeded
	err = s3Client.WaitUntilObjectExists(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destinationKey),
	})
	if err != nil {
		return fmt.Errorf("destination object not confirmed: %w", err)
	}

	// Step 3: Delete original object
	_, err = s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
	})
	if err != nil {
		return fmt.Errorf("failed to delete original object: %w", err)
	}

	return nil
}
