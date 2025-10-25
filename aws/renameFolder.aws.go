package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

// RenameFolder renames a folder by copying all objects under sourcePrefix to destPrefix
// and then deleting the originals. Both prefixes should end in "/".
func RenameFolder(ctx context.Context, bucket, sourcePrefix, destPrefix string) error {
	if !strings.HasSuffix(sourcePrefix, "/") || !strings.HasSuffix(destPrefix, "/") {
		return fmt.Errorf("both source and destination must be folder-like (end in '/')")
	}

	s3Client, err := GetS3Client(ctx)
	if err != nil {
		return fmt.Errorf("failed to get S3 client: %w", err)
	}

	// List all objects under the source prefix
	err = s3Client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(sourcePrefix),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			// Derive new key
			oldKey := *obj.Key
			newKey := strings.Replace(oldKey, sourcePrefix, destPrefix, 1)

			// Copy object
			_, err := s3Client.CopyObject(&s3.CopyObjectInput{
				Bucket:     aws.String(bucket),
				CopySource: aws.String(fmt.Sprintf("%s/%s", bucket, oldKey)),
				Key:        aws.String(newKey),
			})
			if err != nil {
				fmt.Printf("Failed to copy %s to %s: %v\n", oldKey, newKey, err)
				continue
			}

			// Wait for the object to exist at newKey
			err = s3Client.WaitUntilObjectExists(&s3.HeadObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(newKey),
			})
			if err != nil {
				fmt.Printf("Object %s not confirmed at destination: %v\n", newKey, err)
				continue
			}

			// Delete original
			_, err = s3Client.DeleteObject(&s3.DeleteObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(oldKey),
			})
			if err != nil {
				fmt.Printf("Failed to delete original %s: %v\n", oldKey, err)
				continue
			}
		}
		return true
	})

	if err != nil {
		return fmt.Errorf("failed to rename folder: %w", err)
	}

	return nil
}
