package service

import (
	"context"
	"identity_card_ocr/internal/config"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
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

func AWSSdkClient() error {
	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.TODO(),
		awsconfig.WithRegion(config.AWS().Region),
	)
	_ = awsCfg

	return err
}
