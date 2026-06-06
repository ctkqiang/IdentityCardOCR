package main

import (
	"fmt"
	"identity_card_ocr/internal/config"
	"identity_card_ocr/internal/utilities"
)

var SupportedCountries = []config.Country{
	config.CHINA,
	config.MALAYSIA,
	config.US,
}

func main() {
	utilities.LogProgress(
		"IdentityCardOCRService is running... Supported Countries: %v ",
		"IdentityCardOCRService",
		"1.0.0",
		"CST",
		fmt.Sprintf("%v", SupportedCountries),
	)
}
