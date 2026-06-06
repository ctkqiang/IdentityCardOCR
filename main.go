package main

import (
	"context"
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
		"IOCR is running... ",
		"IOCR",
		"1.0.0",
		"CST",
	)

	if err := aws.Init(stateContext); err != nil {
		log.Fatalf("AWS authentication failed: %v", err)
	}
}
