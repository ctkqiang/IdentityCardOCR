package aws

import (
	"context"
	"identity_card_ocr/internal/utilities"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Request struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

// S3Client creates and returns an AWS S3 client instance.
//
// This function obtains the base AWS SDK configuration by calling service.AWSSdkClient,
// and initializes the S3 client based on that configuration. If an error occurs during
// the creation process, it logs the error and returns nil.
//
// Returns:
//   - *s3.Client: Returns the S3 client instance on success; returns nil on failure.
func S3Client() *s3.Client {
	// Lambda 环境中，SDK 会自动从 IAM 角色获取凭证，不需要传任何东西
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		utilities.LogProgress("aws", "S3Client", "Failed to create AWS SDK client", err.Error())
		return nil
	}
	return s3.NewFromConfig(cfg)
}
