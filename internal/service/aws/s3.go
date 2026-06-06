package aws

import (
	"identity_card_ocr/internal/utilities"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Request struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

// S3Client returns an S3 service client constructed from the authenticated
// Account singleton. Init() must have completed successfully before calling
// this function; otherwise it logs an error and returns nil.
func S3Client() *s3.Client {
	if !Ready() {
		utilities.LogProgress("aws", "S3Client", "not authenticated — call Init() first")
		return nil
	}
	return s3.NewFromConfig(GetAccount().Config())
}
