package aws

import "github.com/aws/aws-sdk-go/aws/awserr"

func NotFound(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		return aerr.Code() == "NoSuchKey"
	}
	return false
}
