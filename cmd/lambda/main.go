package main

import (
	"context"
	"log"

	awslambda "github.com/aws/aws-lambda-go/lambda"

	lambdahandler "identity_card_ocr/internal/lambda"
	"identity_card_ocr/internal/service/aws"
)

func main() {
	ctx := context.Background()

	if err := aws.Init(ctx); err != nil {
		log.Fatalf("AWS authentication failed: %v", err)
	}

	if err := aws.EnsureInfrastructure(ctx); err != nil {
		log.Fatalf("Infrastructure provisioning failed: %v", err)
	}

	awslambda.Start(lambdahandler.HandleRequest)
}
