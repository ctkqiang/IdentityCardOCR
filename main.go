package main

import (
	"context"
	"fmt"
	"identity_card_ocr/internal/config"
	"identity_card_ocr/internal/service/aws"
	"identity_card_ocr/internal/utilities"
	"log"
)

var SupportedCountries = []config.Country{
	config.CHINA,
	config.MALAYSIA,
	config.US,
}

func main() {
	stateContext := context.Background()

	utilities.LogProgress(
		"IdentityCardOCRService is running... Supported Countries: %v ",
		"IdentityCardOCRService",
		"1.0.0",
		"CST",
		fmt.Sprintf("%v", SupportedCountries),
	)

	if err := aws.Init(stateContext); err != nil {
		log.Fatalf("AWS authentication failed: %v", err)
	}
}
