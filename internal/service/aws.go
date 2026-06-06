package service

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"

	awsaccount "identity_card_ocr/internal/service/aws"
)

type AWSService string

const (
	Iam         AWSService = "iam"
	S3          AWSService = "s3"
	Textract    AWSService = "textract"
	EventBridge AWSService = "eventbridge"
	Lambda      AWSService = "lambda"
	CloudWatch  AWSService = "cloudwatch"
)

// AWSSdkClient delegates to awsaccount.AWSSdkClient, returning the shared
// AWS SDK config from the authenticated singleton.
//
// Call aws.Init(ctx) once in main() before using this function.
func AWSSdkClient(ctx context.Context) (aws.Config, error) {
	return awsaccount.AWSSdkClient(ctx)
}
