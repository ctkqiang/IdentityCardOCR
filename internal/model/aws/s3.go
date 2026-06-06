package aws

import (
	"context"
	"identity_card_ocr/internal/service"
	"identity_card_ocr/internal/utilities"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Request struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

// S3Client 创建并返回一个 AWS S3 客户端实例。
//
// 该函数通过调用 service.AWSSdkClient 获取基础的 AWS SDK 配置，
// 并基于该配置初始化 S3 客户端。如果创建过程中发生错误，
// 将记录错误日志并返回 nil。
//
// 返回值:
//   - *s3.Client: 成功时返回 S3 客户端实例；失败时返回 nil。
func S3Client() *s3.Client {
	client, err := service.AWSSdkClient(context.Background())
	if err != nil {
		utilities.LogProgress("aws", "S3Client", "Failed to create AWS SDK client", err.Error())
		return nil
	}

	return s3.NewFromConfig(client)
}
