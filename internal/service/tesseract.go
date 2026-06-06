package service

import (
	"identity_card_ocr/internal/config"
	"identity_card_ocr/internal/model"
	"identity_card_ocr/internal/utilities"
	"sync"

	"github.com/otiai10/gosseract/v2"
)

var (
	countryLocaleOnce sync.Once
	countryLocaleMap  map[config.Country]config.Locale
)

// GetTesseractClient allocates and returns a new gosseract OCR client.
// The caller must call Close() on the returned client to release native
// Tesseract resources.
func GetTesseractClient() *gosseract.Client {
	return gosseract.NewClient()
}

// ExtractTextFromIdentityDocument runs Tesseract OCR on the image at imagePath
// using the locale and segmentation mode appropriate for country, then parses
// the raw text into a structured DocumentInfo.
//
// Supported country values:
//   - config.CHINA:    chi_sim locale, PSM_SINGLE_BLOCK, Chinese ID card parsing
//   - config.MALAYSIA: eng locale, default PSM, Malaysian MyKad parsing
//   - config.US:       eng locale, default PSM, no country-specific parsing
//
// The returned DocumentInfo always carries RawText; structured fields are
// populated only when the parser successfully identifies a known document format.
func ExtractTextFromIdentityDocument(imagePath string, country config.Country) (model.DocumentInfo, error) {
	var locale string

	locale = string(setLocale(country))
	client := GetTesseractClient()

	defer client.Close()

	err := client.SetImage(imagePath)
	if err != nil {
		utilities.LogProgress(
			"tesseract",
			"ExtractTextFromIdentityDocument",
			"Failed",
			"Failed to set image",
			err.Error(),
		)

		return model.DocumentInfo{}, err
	}

	client.SetLanguage(locale)
	client.SetWhitelist("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	if country == config.CHINA {
		client.SetPageSegMode(gosseract.PSM_SINGLE_BLOCK)
	}

	text, err := client.Text()
	if err != nil {
		utilities.LogProgress(
			"tesseract",
			"ExtractTextFromIdentityDocument",
			"Failed",
			"Failed to extract text from image",
			err.Error(),
		)

		return model.DocumentInfo{}, err
	}

	return ParseOCR(model.DocumentInfo{
		RawText: text,
	}, country), nil
}

// setLocale maps a Country to its corresponding Locale using a lazily-initialized lookup table.
// The table is built once via sync.Once; subsequent calls are O(1) map lookups.
func setLocale(c config.Country) config.Locale {
	countryLocaleOnce.Do(func() {
		countryLocaleMap = map[config.Country]config.Locale{
			config.CHINA: config.CHINESE,
		}
	})

	if locale, ok := countryLocaleMap[c]; ok {
		return locale
	}
	return config.ENGLISH
}
