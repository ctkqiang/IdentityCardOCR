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
	} else {
		utilities.LogProgress(
			"IOCR is running in [Development Mode]... ",
			"IOCR",
			"1.0.0",
			"CST",
		)
	}
}
