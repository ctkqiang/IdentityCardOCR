package main

import (
	"context"
	"identity_card_ocr/internal/config"
	"identity_card_ocr/internal/service/aws"
	"identity_card_ocr/internal/utilities"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
)

var (
	SupportedCountries = []config.Country{
		config.CHINA,
		config.MALAYSIA,
	}
	IsProduction = os.Getenv("IS_PRODUCTION") == "true"
)

func main() {
	stateContext := context.Background()

	if IsProduction {
		lambda.Start(aws.LambdaHandler)

		utilities.LogProgress(
			"IOCR is running in [Production Mode]... ",
			"IOCR",
			"1.0.0",
			"CST",
		)

		if err := aws.Init(stateContext); err != nil {
			log.Fatalf("AWS authentication failed: %v", err)
		}

		// Get Image from S3  each obj
		// then parse OCR text
		// then store result to DynamoDB as User Identity
		// EVENT must show passed or failed , only passed insert into DB

		// Flow
		// FROM client UPload img(wedonthnalde client) -> Lambda fetch s3 and handle each obj ,
		// the passed insert into DB with passed event else the failed insert // into another table that mention failed?
		// Idk ....

		// If no s3 object just retut nno object message
		// NEEDED _LAMBDA_SERVER_PORT AWS_LAMBDA_RUNTIME_API

		// and more features

	} else {
		utilities.LogProgress(
			"IOCR is running in [Development Mode]... ",
			"IOCR",
			"1.0.0",
			"CST",
		)
	}
}
