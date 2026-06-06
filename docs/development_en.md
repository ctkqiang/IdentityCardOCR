# Development Guide

## Prerequisites

- Go 1.26 or later
- Tesseract OCR 5.x
- Tesseract language data: `chi_sim` (Chinese Simplified), `eng` (English)
- macOS or Linux (Tesseract CGo bindings)

### macOS Setup

```bash
brew install tesseract tesseract-lang
```

### Linux Setup

```bash
sudo apt-get install -y tesseract-ocr tesseract-ocr-chi-sim tesseract-ocr-eng
```

## Local Development

### Dev Mode CLI

When `IS_PRODUCTION` is not set to `"true"`, the application starts an interactive terminal session:

```bash
go run main.go
```

Output:
```
========================================
  IdentityCardOCR — Dev Mode CLI
========================================
Supported countries: china, malaysia, us
========================================

Enter image path (or 'exit' to quit): sample/china/identity_card.png
Enter country (china/malaysia/us): china

Processing sample/china/identity_card.png (china)...

----------------------------------------
  Extracted Identity Information
----------------------------------------
  ID Number    : 350125199006081234
  Name         : ZHANGSAN
  Nationality  : 福建省福州市永泰县
  Date of Birth: 1990-06-08
  Sex          : 男
  Expiry Date  : 2020-06-08 ~ 2040-06-08
----------------------------------------
  Raw OCR Text :
    姓名 ZHANGSAN 性别 男 民族 汉 ...
----------------------------------------
  [GB11643-1999 Validation: PASSED]
  Region       : 福建省福州市永泰县
  Check Digit  : 4
----------------------------------------
```

### Running Tests

```bash
# Run OCR integration tests
make test

# Run with verbose output
go test -v ./test/ -count=1
```

The test suite processes sample images in `sample/` and validates:
- Chinese ID card: full GB11643-1999 checksum, DOB format, sex consistency, name extraction
- Chinese passport: raw OCR text extraction
- Malaysian MyKad: 12-digit pattern, DOB derivation, sex derivation
- Malaysian passport: raw OCR text extraction

### Build

```bash
make build
# Output: bin/identity_card_ocr
```

## Project Layout Conventions

- `internal/` contains all application logic; nothing outside this package is importable by external modules
- Each package has a single responsibility (config, event, pipeline, etc.)
- The `service/aws` package owns the AWS authentication singleton and infrastructure provisioning
- The `service/dynamodb` package is the sole DynamoDB data access layer
- The `utilities` package has zero internal dependencies — it can be extracted to a shared library
