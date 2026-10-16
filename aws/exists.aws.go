package aws

import (
	"context"
	"errors"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
)

// ErrDestinationExists is returned by RenameObject / RenameFolder when the
// destination is taken and overwriting wasn't asked for. Handlers answer 409.
//
// openbucket-go overwrites on CopyObject, as S3 does (openbucket-go#23).
// Before that a rename onto an existing name failed by accident; this keeps
// it from silently replacing someone's file — or merging two folders — now.
var ErrDestinationExists = errors.New("destination already exists")

// objectExists reports whether key exists in bucket.
func objectExists(ctx context.Context, c *s3.S3, bucket, key string) (bool, error) {
	_, err := c.HeadObjectWithContext(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err == nil {
		return true, nil
	}
	var rf awserr.RequestFailure
	if errors.As(err, &rf) && rf.StatusCode() == http.StatusNotFound {
		return false, nil
	}
	return false, err
}

// prefixInUse reports whether any key starts with prefix.
func prefixInUse(ctx context.Context, c *s3.S3, bucket, prefix string) (bool, error) {
	out, err := c.ListObjectsV2WithContext(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket), Prefix: aws.String(prefix), MaxKeys: aws.Int64(1),
	})
	if err != nil {
		return false, err
	}
	return len(out.Contents) > 0 || len(out.CommonPrefixes) > 0, nil
}
