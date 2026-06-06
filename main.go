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
		runDevCLI(stateContext)
	}
}

func runDevCLI(ctx context.Context) {
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

func printDocument(doc model.DocumentInfo, country config.Country) {
	fmt.Println("----------------------------------------")
	fmt.Println("  Extracted Identity Information")
	fmt.Println("----------------------------------------")

	if doc.IDNumber != "" {
		fmt.Printf("  ID Number    : %s\n", doc.IDNumber)
	}
	if doc.Name != "" {
		fmt.Printf("  Name         : %s\n", doc.Name)
	}
	if doc.Nationality != "" {
		fmt.Printf("  Nationality  : %s\n", doc.Nationality)
	}
	if doc.DateOfBirth != "" {
		fmt.Printf("  Date of Birth: %s\n", doc.DateOfBirth)
	}
	if doc.Sex != "" {
		fmt.Printf("  Sex          : %s\n", doc.Sex)
	}
	if doc.Address != "" {
		fmt.Printf("  Address      : %s\n", doc.Address)
	}
	if doc.ExpiryDate != "" {
		fmt.Printf("  Expiry Date  : %s\n", doc.ExpiryDate)
	}
	if doc.RawText != "" {
		fmt.Println("----------------------------------------")
		fmt.Printf("  Raw OCR Text :\n    %s\n", doc.RawText)
	}
	fmt.Println("----------------------------------------")

	if country == config.CHINA && doc.IDNumber != "" {
		info := utilities.ParseIDInfo(doc.IDNumber)
		if info != nil {
			fmt.Println("  [GB11643-1999 Validation: PASSED]")
			if info.Region != "" {
				fmt.Printf("  Region       : %s\n", info.Region)
			}
			fmt.Printf("  Check Digit  : %s\n", info.CheckDigit)
			fmt.Println("----------------------------------------")
		}
	}

	if country == config.MALAYSIA && doc.IDNumber != "" {
		bp := utilities.MyKadBirthPlace(doc.IDNumber[6:8])
		if bp != "" {
			fmt.Printf("  Birth Place  : %s\n", bp)
		}
		bm := utilities.MyKadBirthMonth(doc.IDNumber[2:4])
		if bm != "" {
			fmt.Printf("  Birth Month  : %s\n", bm)
		}
		fmt.Println("----------------------------------------")
	}
}
