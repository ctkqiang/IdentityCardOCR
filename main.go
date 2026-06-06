package main

import (
	"bufio"
	"context"
	"fmt"
	"identity_card_ocr/internal/config"
	lambdahandler "identity_card_ocr/internal/lambda"
	"identity_card_ocr/internal/model"
	"identity_card_ocr/internal/service"
	"identity_card_ocr/internal/service/aws"
	"identity_card_ocr/internal/utilities"
	"log"
	"os"
	"strings"

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
		utilities.LogProgress(
			"IOCR is running in [Production Mode]... ",
			"IOCR",
			"1.0.0",
			"CST",
		)

		if err := aws.Init(stateContext); err != nil {
			log.Fatalf("AWS authentication failed: %v", err)
		}

		// Ensure S3 bucket, DynamoDB tables, and EventBridge bus exist.
		// Creates any missing resources before Lambda starts serving requests.
		if err := aws.EnsureInfrastructure(stateContext); err != nil {
			log.Fatalf("Infrastructure provisioning failed: %v", err)
		}

		// Image uploaded from client -> S3 trigger -> Lambda retrieves S3 object
		// Then use Tesseract OCR to parse text -> Extract ID card fields
		// Success -> Store result to DynamoDB user_identity table + emit processing.completed event
		// Failure -> Store to DynamoDB failed_records table + emit processing.failed event
		// If no S3 object -> Return "no objects to process" message
		lambda.Start(lambdahandler.HandleRequest)

	} else {
		runDevCLI()
	}
}

// runDevCLI starts an interactive terminal session for local OCR testing.
// The user provides an image path and country code; the function runs OCR,
// parses the result, and prints extracted identity fields to stdout.
// Type "exit" or "quit" at the image prompt to stop the loop.
func runDevCLI() {
	scanner := bufio.NewScanner(os.Stdin)

	utilities.LogProgress(
		"IOCR is running in [Development Mode]... ",
		"IOCR",
		"1.0.0",
		"CST",
	)

	fmt.Print("\n")
	fmt.Println("========================================")
	fmt.Println("  IdentityCardOCR — Dev Mode CLI")
	fmt.Println("========================================")
	fmt.Println("Supported countries: china, malaysia, us")
	fmt.Println("========================================")
	fmt.Print("\n")

	for {
		fmt.Print("Enter image path (or 'exit' to quit): ")
		if !scanner.Scan() {
			break
		}
		imagePath := strings.TrimSpace(scanner.Text())
		if imagePath == "" {
			continue
		}
		if strings.EqualFold(imagePath, "exit") || strings.EqualFold(imagePath, "quit") {
			fmt.Println("Goodbye.")
			break
		}

		if _, err := os.Stat(imagePath); os.IsNotExist(err) {
			fmt.Printf("File not found: %s\n\n", imagePath)
			continue
		}

		fmt.Print("Enter country (china/malaysia/us): ")
		if !scanner.Scan() {
			break
		}
		countryStr := strings.ToLower(strings.TrimSpace(scanner.Text()))

		country, err := config.CountryFromString(countryStr)
		if err != nil {
			fmt.Printf("Unknown country: %s (supported: china, malaysia, us)\n\n", countryStr)
			continue
		}

		fmt.Printf("\nProcessing %s (%s)...\n\n", imagePath, country.String())

		doc, err := service.ExtractTextFromIdentityDocument(imagePath, country)
		if err != nil {
			fmt.Printf("OCR failed: %v\n\n", err)
			continue
		}

		printDocument(doc, country)
		fmt.Print("\n")
	}
}

// ANSI terminal escapes for the CLI card UI.
const (
	cBold   = "\033[1m"
	cDim    = "\033[2m"
	cReset  = "\033[0m"
	cGreen  = "\033[32m"
	cRed    = "\033[31m"
	cYellow = "\033[33m"
	cCyan   = "\033[36m"
	cWhite  = "\033[37m"
	cGray   = "\033[90m"
)

// printDocument renders the extracted identity fields as a styled terminal card.
func printDocument(doc model.DocumentInfo, country config.Country) {
	labelWidth := 16

	docType := documentTypeLabel(country, doc.IDNumber)
	headerColor := headerColorForCountry(country)

	fmt.Print("\n")
	fmt.Printf("  %s╭──────────────────────────────────────────────╮%s\n", cGray, cReset)
	fmt.Printf("  %s│%s  %s%-44s%s %s│%s\n", cGray, cReset, headerColor+cBold, docType, cReset, cGray, cReset)
	fmt.Printf("  %s├──────────────────────────────────────────────┤%s\n", cGray, cReset)

	field := func(label, value string) {
		if value == "" {
			return
		}
		fmt.Printf("  %s│%s  %-*s %s%s%s\n", cGray, cReset, labelWidth, cDim+label+":", cReset, cWhite, value)
	}

	field("ID Number", doc.IDNumber)
	field("Name", doc.Name)
	field("Nationality", doc.Nationality)
	field("Date of Birth", doc.DateOfBirth)
	field("Sex", doc.Sex)
	field("Address", doc.Address)

	if doc.ExpiryDate != "" {
		expiryColor := cWhite
		labelColor := cDim
		fmt.Printf("  %s│%s  %-*s %s%s%s\n", cGray, cReset, labelWidth, labelColor+"Expiry Date:", cReset, expiryColor, doc.ExpiryDate)
	}

	fmt.Printf("  %s├──────────────────────────────────────────────┤%s\n", cGray, cReset)

	// Validation block — country-specific checks.
	validationBlock(country, doc)

	fmt.Printf("  %s╰──────────────────────────────────────────────╯%s\n", cGray, cReset)

	// Raw OCR text below the card.
	if doc.RawText != "" {
		fmt.Printf("\n  %s┌─ Raw OCR Output %s\n", cGray, strings.Repeat("─", 54))
		for _, line := range strings.Split(doc.RawText, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fmt.Printf("  %s│%s %s%s%s\n", cGray, cReset, cDim, line, cReset)
		}
		fmt.Printf("  %s└%s%s\n\n", cGray, strings.Repeat("─", 59), cReset)
	}
}

func validationBlock(country config.Country, doc model.DocumentInfo) {
	switch {
	case country == config.CHINA && doc.IDNumber != "":
		info := utilities.ParseIDInfo(doc.IDNumber)
		if info != nil {
			valid := fmt.Sprintf("%s%sVALID%s", cGreen+cBold, "  PASS  ", cReset)
			fmt.Printf("  %s│%s  GB11643-1999    %-30s%s│%s\n", cGray, cReset, valid, cGray, cReset)
			meta("Region", info.Region)
			meta("Check Digit", info.CheckDigit)
		} else {
			fail := fmt.Sprintf("%s%sFAIL%s", cRed+cBold, "  FAIL  ", cReset)
			fmt.Printf("  %s│%s  GB11643-1999    %-30s%s│%s\n", cGray, cReset, fail, cGray, cReset)
		}

	case country == config.MALAYSIA && doc.IDNumber != "":
		bp := utilities.MyKadBirthPlace(doc.IDNumber[6:8])
		if bp != "" {
			meta("Birth Place", bp)
		}
		bm := utilities.MyKadBirthMonth(doc.IDNumber[2:4])
		if bm != "" {
			meta("Birth Month", bm)
		}

	default:
		if doc.IDNumber == "" {
			fmt.Printf("  %s│%s  %s%-44s%s%s│%s\n", cGray, cReset, cRed, "  No ID number extracted", cReset, cGray, cReset)
		}
	}
}

func meta(label, value string) {
	if value == "" {
		return
	}
	fmt.Printf("  %s│%s  %-16s %s%s%s\n", cGray, cReset, cDim+label+":", cReset, cWhite, value)
}

func documentTypeLabel(country config.Country, idNumber string) string {
	switch country {
	case config.CHINA:
		if idNumber != "" && len(idNumber) == 18 {
			return "CHINESE IDENTITY CARD"
		}
		return "CHINESE DOCUMENT"
	case config.MALAYSIA:
		if idNumber != "" && len(idNumber) == 12 {
			return "MALAYSIAN MyKad"
		}
		return "MALAYSIAN DOCUMENT"
	default:
		return "IDENTITY DOCUMENT"
	}
}

func headerColorForCountry(country config.Country) string {
	switch country {
	case config.CHINA:
		return cRed
	case config.MALAYSIA:
		return cYellow
	default:
		return cCyan
	}
}
