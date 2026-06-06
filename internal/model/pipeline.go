package model

import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/textract"
)

type Pipeline struct {
	AWSS3Client    *s3.Client
	TextractClient *textract.Client
}
