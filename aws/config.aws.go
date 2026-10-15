package aws

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/aidenappl/openbucket-api/cache"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/tools"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// GetSessionFromContext extracts session claims from context.
func GetSessionFromContext(ctx context.Context) (*tools.SessionClaims, error) {
	sessionData := middleware.GetSession(ctx)
	if sessionData == nil {
		return nil, fmt.Errorf("no session found in context")
	}
	return sessionData, nil
}

// storageHTTPClient is shared by every S3 session.
//
// ⚠️ The SDK default is http.DefaultClient, which keeps 2 idle connections
// per host. The storage servers are reached through Cloudflare
// (cdn.appleby.cloud, cdn.trailblaze.to), so a 100-key bulk HEAD opened ~98
// fresh TCP+TLS connections every time, and every listing page after a quiet
// spell paid a handshake. This pool keeps them warm and negotiates HTTP/2,
// which multiplexes concurrent requests over one connection.
//
// No overall Timeout: downloads stream objects of any size.
var storageHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   128,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

// CreateAWSSession creates an AWS session from context session claims.
// It verifies that the session belongs to the authenticated user.
// Sessions are cached by session ID for 5 minutes.
func CreateAWSSession(ctx context.Context) (*session.Session, error) {
	sessionClaims, err := GetSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Verify the session belongs to the authenticated user
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, fmt.Errorf("no authenticated user in context")
	}
	if sessionClaims.UserID != int64(userID) {
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
		HTTPClient:       storageHTTPClient,
	}

	if sessionClaims.AccessKey != nil && sessionClaims.SecretKey != nil {
		config.Credentials = credentials.NewStaticCredentials(
			*sessionClaims.AccessKey,
			*sessionClaims.SecretKey,
			"",
		)
	}

	sess, err := session.NewSession(config)
	if err != nil {
		return nil, err
	}

	cache.SetAWSSession(sessionClaims.SessionID, sess)
	return sess, nil
}

// GetS3Client creates an S3 client from context.
func GetS3Client(ctx context.Context) (*s3.S3, error) {
	sess, err := CreateAWSSession(ctx)
	if err != nil {
		return nil, err
	}
	return s3.New(sess), nil
}

// GetUploader creates an S3 uploader from context.
func GetUploader(ctx context.Context) (*s3manager.Uploader, error) {
	sess, err := CreateAWSSession(ctx)
	if err != nil {
		return nil, err
	}
	return s3manager.NewUploader(sess), nil
}

// GetDownloader creates an S3 downloader from context.
func GetDownloader(ctx context.Context) (*s3manager.Downloader, error) {
	sess, err := CreateAWSSession(ctx)
	if err != nil {
		return nil, err
	}
	return s3manager.NewDownloader(sess), nil
}
