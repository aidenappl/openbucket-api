package aws

import (
	"net/http"
	"strings"

	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aws/aws-sdk-go/aws/awserr"
)

func CheckError(err error, w http.ResponseWriter, r *http.Request) bool {
	if err == nil {
		return false
	}
	if awsErr, ok := err.(awserr.Error); ok {
		switch awsErr.Code() {
		case "AccessDenied", "InvalidAccessKeyId", "SignatureDoesNotMatch", "Forbidden":
			responder.SendError(w, http.StatusUnauthorized, "Unauthorized", err)
			return true
		case "NoSuchBucket", "NoSuchKey", "NotFound":
			responder.SendError(w, http.StatusNotFound, "Resource not found", err)
			return true
		case "RequestError":
			if strings.Contains(err.Error(), "connect: connection refused") {
				responder.SendError(w, http.StatusServiceUnavailable, "Service unavailable", err)
				return true
			}
			responder.SendError(w, http.StatusBadRequest, "Bad request", err)
			return true
		default:
			responder.SendError(w, http.StatusInternalServerError, "AWS error", err)
			return true
		}
	}
	responder.SendError(w, http.StatusInternalServerError, "Unknown error", err)
	return true
}
