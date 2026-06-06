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

func GetTesseractClient() *gosseract.Client {
	return gosseract.NewClient()
}

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
