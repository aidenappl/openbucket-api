package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func ListFolders(bucket string, prefix string) ([]string, error) {
	out, err := s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"), // key for folders
	})
	if err != nil {
		return nil, err
	}

	var folders []string
	for _, cp := range out.CommonPrefixes {
		folders = append(folders, *cp.Prefix)
	}

	if len(folders) == 0 {
		return nil, nil
	}
	return folders, nil

}
