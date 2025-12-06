package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

var (
	ACLPublicRead  = "public-read"
	ACLPrivate     = "private"
	ACLBucketOwner = "bucket-owner-full-control"
)

func ModifyObjectACL(ctx context.Context, bucket, key string, acl string) error {

	if bucket == "" {
		return fmt.Errorf("bucket name cannot be empty")
	}
	if key == "" {
		return fmt.Errorf("object key cannot be empty")
	}
	if acl == "" {
		return fmt.Errorf("ACL cannot be empty")
	} else if acl != ACLPublicRead && acl != ACLPrivate && acl != ACLBucketOwner {
		return fmt.Errorf("invalid ACL value: %s, must be one of: %s, %s, %s",
			acl, ACLPublicRead, ACLPrivate, ACLBucketOwner)
	}

	s3Client, err := GetS3Client(ctx)
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %v", err)
	}

	_, err = s3Client.PutObjectAcl(&s3.PutObjectAclInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		ACL:    aws.String(acl),
	})
	if err != nil {
		return err
	}

	return nil
}

func GetObjectACL(ctx context.Context, bucket, key string) (*s3.GetObjectAclOutput, error) {
	if bucket == "" {
		return nil, fmt.Errorf("bucket name cannot be empty")
	}

	if key == "" {
		return nil, fmt.Errorf("object key cannot be empty")
	}

	s3Client, err := GetS3Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %v", err)
	}

	output, err := s3Client.GetObjectAcl(&s3.GetObjectAclInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	return output, nil
}
