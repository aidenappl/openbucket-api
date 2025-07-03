package aws

import (
	"github.com/aidenappl/openbucket-api/env"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var (
	sess = session.Must(session.NewSession(&aws.Config{
		Endpoint:         aws.String(env.Endpoint),
		S3ForcePathStyle: aws.Bool(true),
	}))
	uploader   = s3manager.NewUploader(sess)
	downloader = s3manager.NewDownloader(sess)
	s3Client   = s3.New(sess)
)
