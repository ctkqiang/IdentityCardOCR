package main

import (
	awslambda "github.com/aws/aws-lambda-go/lambda"

	lambdahandler "identity_card_ocr/internal/lambda"
)

func main() {
	awslambda.Start(lambdahandler.HandleRequest)
}
