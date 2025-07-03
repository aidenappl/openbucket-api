package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

var (
	ACLPublicRead  = "public-read"
	ACLPrivate     = "private"
	ACLBucketOwner = "bucket-owner-full-control"
)

func ModifyACL(bucket, key string, acl string) error {

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

	svc := s3.New(sess)

	_, err := svc.PutObjectAcl(&s3.PutObjectAclInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		ACL:    aws.String(acl),
	})
	if err != nil {
		return fmt.Errorf("failed to modify object ACL: %v", err)
	}

	return nil
}
