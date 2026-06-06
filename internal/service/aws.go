package service

import (
	"context"
	"identity_card_ocr/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
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

// AWSSdkClient 加载并返回默认的 AWS SDK 配置。
//
// 参数:
//   - ctx: 上下文对象，用于控制请求的生命周期和取消操作。
//
// 返回值:
//   - aws.Config: 成功时返回加载好的 AWS 配置对象。
//   - error: 如果加载配置过程中发生错误，则返回相应的错误信息；否则返回 nil。
func AWSSdkClient(ctx context.Context) (aws.Config, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(config.AWS().Region),
	)
	if err != nil {
		return aws.Config{}, err
	}

	return awsCfg, nil
}
