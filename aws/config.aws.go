package aws

import (
	"context"
	"fmt"

	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/tools"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type contextKey string

const SessionContextKey contextKey = "session"

// GetSessionFromContext extracts session claims from context
func GetSessionFromContext(ctx context.Context) (*tools.SessionClaims, error) {
	sessionData := middleware.GetSession(ctx)
	if sessionData == nil {
		return nil, fmt.Errorf("no session found in context")
	}

	return sessionData, nil
}

// CreateAWSSession creates an AWS session from context session claims
func CreateAWSSession(ctx context.Context) (*session.Session, error) {
	sessionClaims, err := GetSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	config := &aws.Config{
		Endpoint:         aws.String(sessionClaims.Endpoint),
		Region:           aws.String(sessionClaims.Region),
		S3ForcePathStyle: aws.Bool(true),
	}

	// Use credentials from session if provided, otherwise fall back to env vars
	if sessionClaims.AccessKey != nil && sessionClaims.SecretKey != nil {
		panic("failed to get session information")
	}

	return session.NewSession(config)
}

// GetS3Client creates an S3 client from context
func GetS3Client(ctx context.Context) (*s3.S3, error) {
	sess, err := CreateAWSSession(ctx)
	if err != nil {
		return nil, err
	}
	return s3.New(sess), nil
}

// GetUploader creates an S3 uploader from context
func GetUploader(ctx context.Context) (*s3manager.Uploader, error) {
	sess, err := CreateAWSSession(ctx)
	if err != nil {
		return nil, err
	}
	return s3manager.NewUploader(sess), nil
}

// GetDownloader creates an S3 downloader from context
func GetDownloader(ctx context.Context) (*s3manager.Downloader, error) {
	sess, err := CreateAWSSession(ctx)
	if err != nil {
		return nil, err
	}
	return s3manager.NewDownloader(sess), nil
}
