package aws

import (
	"context"
	"fmt"

	forta "github.com/aidenappl/go-forta"
	"github.com/aidenappl/openbucket-api/cache"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/tools"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
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

// CreateAWSSession creates an AWS session from context session claims.
// It also verifies that the session belongs to the authenticated Forta user.
// Sessions are cached by session ID for 5 minutes.
func CreateAWSSession(ctx context.Context) (*session.Session, error) {
	sessionClaims, err := GetSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Verify the session belongs to the authenticated user
	fortaID, ok := forta.GetFortaIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no authenticated user in context")
	}
	if sessionClaims.FortaUserID != fortaID {
		return nil, fmt.Errorf("session does not belong to authenticated user")
	}

	// Check AWS session cache
	if cached, ok := cache.GetAWSSession(sessionClaims.SessionID); ok {
		return cached, nil
	}

	config := &aws.Config{
		Endpoint:         aws.String(sessionClaims.Endpoint),
		Region:           aws.String(sessionClaims.Region),
		S3ForcePathStyle: aws.Bool(true),
	}

	if sessionClaims.AccessKey != nil && sessionClaims.SecretKey != nil {
		config.Credentials = credentials.NewStaticCredentials(
			*sessionClaims.AccessKey,
			*sessionClaims.SecretKey,
			"", // token
		)
	}

	sess, err := session.NewSession(config)
	if err != nil {
		return nil, err
	}

	// Cache the session
	cache.SetAWSSession(sessionClaims.SessionID, sess)
	return sess, nil
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
