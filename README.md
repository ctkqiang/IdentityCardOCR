# IdentityCardOCR

![](./docs/logo.svg)

[English](#english) | [中文](#中文)

Production-grade, open-source AWS Lambda service for OCR-based identity document extraction. Supports Chinese Resident Identity Cards (中华人民共和国居民身份证) and Malaysian MyKad/MyPR documents with automatic AWS infrastructure provisioning.

## Architecture Overview

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   S3 Bucket   │────▶│ AWS Lambda   │────▶│  DynamoDB    │
│ (image upload)│     │ (OCR engine) │     │ (structured) │
└──────────────┘     └──────┬───────┘     └──────────────┘
                            │
                            ▼
                     ┌──────────────┐
                     │ EventBridge  │
                     │ (event bus)  │
                     └──────┬───────┘
                            │
                            ▼
                     ┌──────────────┐
                     │  Downstream  │
                     │  Consumers   │
                     └──────────────┘
```

**Flow:** Client uploads image to S3 → S3 event triggers Lambda → Tesseract OCR extracts text → Parser extracts structured fields → Passed results stored in DynamoDB `user_identity` table + `processing.completed` event emitted → Failed attempts stored in DynamoDB `failed_records` table + `processing.failed` event emitted.

## Supported Documents

| Country               | Document Type                       | OCR Locale | Parser                                |
| --------------------- | ----------------------------------- | ---------- | ------------------------------------- |
| China (`china`)       | 居民身份证 (Resident Identity Card) | `chi_sim`  | GB11643-1999 checksum validation      |
| China (`china`)       | 中国护照 (Chinese Passport)         | `chi_sim`  | Raw text extraction                   |
| Malaysia (`malaysia`) | MyKad / MyPR                        | `eng`      | 12-digit pattern + DOB/Sex derivation |
| Malaysia (`malaysia`) | Malaysian Passport                  | `eng`      | Raw text extraction                   |

## Project Structure

```
IdentityCardOCR/
├── main.go                          # Root entry point (prod Lambda / dev CLI dispatch)
├── aws-config.yml                   # AWS resource configuration (region, bucket, tables)
├── .env                             # AWS credentials (gitignored)
├── .env.example                     # Credential template
├── Makefile                         # Build targets (test, build, clean)
├── go.mod / go.sum                  # Go module dependencies
│
├── cmd/
│   └── lambda/
│       └── main.go                  # Standalone Lambda binary entry point
│
├── internal/
│   ├── config/
│   │   └── config.go                # YAML + .env configuration loader
│   │
│   ├── model/
│   │   ├── document_info.go         # DocumentInfo, ChineseIdentityCardInfo types
│   │   ├── pipeline.go              # OCR pipeline struct (S3 + Textract clients)
│   │   └── response.go              # Lambda response envelope
│   │
│   ├── event/
│   │   ├── types.go                 # Event envelope, payload types, UUID generation
│   │   ├── store.go                 # S3-backed append-only event log
│   │   ├── bridge.go                # EventBridge publishing (single + batch)
│   │   └── event.go                 # LambdaEvent type
│   │
│   ├── pipeline/
│   │   └── pipeline.go              # Event-driven OCR pipeline orchestrator
│   │
│   ├── lambda/
│   │   └── handler.go               # Main Lambda S3 event handler
│   │
│   ├── service/
│   │   ├── aws.go                   # AWS service type constants + SDK config delegate
│   │   ├── tesseract.go             # Tesseract OCR integration (gosseract v2)
│   │   ├── parser.go                # OCR text → structured DocumentInfo parser
│   │   │
│   │   ├── aws/
│   │   │   ├── account.go           # AWS authentication singleton (STS verified)
│   │   │   ├── infra.go             # Infrastructure auto-provisioning
│   │   │   ├── s3.go                # S3 client factory
│   │   │   └── lambda.go            # Package documentation (handler moved to internal/lambda/)
│   │   │
│   │   └── dynamodb/
│   │       └── client.go            # DynamoDB client (PutUserIdentity, PutFailedRecord, GetUserIdentity)
│   │
│   └── utilities/
│       ├── logger.go                # Structured CloudWatch logging framework
│       └── validator.go             # Chinese ID + MyKad validation logic
│
├── sample/
│   ├── china/
│   │   ├── identity_card.png        # Sample Chinese ID card
│   │   └── passport.png             # Sample Chinese passport
│   └── malaysia/
│       ├── identity_card.png        # Sample Malaysian MyKad
│       └── passport.png             # Sample Malaysian passport
│
├── test/
│   └── id_test.go                   # OCR + parser integration tests
│
└── docs/                            # Comprehensive documentation
```

## Quick Start

### Prerequisites

- Go 1.26+
- Tesseract OCR 5.x with language data (`chi_sim`, `eng`)
- AWS account with credentials configured

### Local Development (Dev Mode)

```bash
# Install Tesseract (macOS)
brew install tesseract tesseract-lang

# Ensure IS_PRODUCTION is NOT set to "true" in .env
# Run in dev mode — interactive CLI scanner
go run main.go
```

Dev mode is an interactive CLI: you provide an image path and country, the tool runs OCR and prints the extracted identity fields to your terminal. No AWS resources needed.

### Running Tests

```bash
make test
```

### Production Build

```bash
# Set production mode
export IS_PRODUCTION=true

# Build Lambda binary for ARM64
GOOS=linux GOARCH=arm64 go build -o bootstrap ./cmd/lambda/main.go
```

## Configuration

All AWS resource names and region are defined in `aws-config.yml`:

```yaml
environment:
  region: ap-east-1
  profile: default
  s3:
    bucket: identity-card-ocr
    path: identity/
  eventbridge:
    bus_name: identity-card-ocr-bus
    source: identity-card-ocr
  dynamodb:
    user_identity_table: identity-card-ocr-users
    failed_records_table: identity-card-ocr-failed
```

Credentials are loaded from `.env` (highest priority) or the AWS SDK default credential chain (IAM role for Lambda, `~/.aws/credentials` for local dev).

## Infrastructure Auto-Provisioning

On cold start, the application automatically verifies and creates missing AWS resources:

1. **S3 Bucket** — HeadBucket → CreateBucket (with region-specific LocationConstraint)
2. **DynamoDB Tables** — DescribeTable → CreateTable with `PAY_PER_REQUEST` billing → wait for ACTIVE
3. **EventBridge Bus** — DescribeEventBus → CreateEventBus

All checks are idempotent — safe to run on every cold start without side effects.

## Event System

The application uses an event-driven architecture with three event types:

| Event Type             | Detail Payload                                                                               | When Emitted              |
| ---------------------- | -------------------------------------------------------------------------------------------- | ------------------------- |
| `document.submitted`   | `DocumentSubmittedPayload` (image_path, s3_bucket, s3_key, country)                          | Client submits a document |
| `processing.completed` | `ProcessingCompletedPayload` (id_number, name, nationality, dob, sex, expiry_date, raw_text) | OCR + parsing succeeds    |
| `processing.failed`    | `ProcessingFailedPayload` (error, phase)                                                     | Any processing step fails |

Events are durably stored in S3 (`{prefix}/events/{documentID}/{timestamp}.json`) and published to EventBridge for downstream consumers.

## DynamoDB Data Model

### `identity-card-ocr-users` (passed)

| Field              | Type   | Description                |
| ------------------ | ------ | -------------------------- |
| `document_id` (PK) | String | S3 object key              |
| `id_number`        | String | Extracted ID number        |
| `name`             | String | Cardholder name            |
| `date_of_birth`    | String | YYYY-MM-DD                 |
| `sex`              | String | 男/女 or LELAKI/PEREMPUAN  |
| `nationality`      | String | Region or nationality      |
| `expiry_date`      | String | Document expiry (if found) |
| `raw_text`         | String | Raw OCR output             |
| `country`          | String | china / malaysia / us      |
| `created_at`       | String | RFC3339 timestamp          |

### `identity-card-ocr-failed` (failed)

| Field              | Type   | Description                |
| ------------------ | ------ | -------------------------- |
| `document_id` (PK) | String | S3 object key              |
| `error`            | String | Error message              |
| `phase`            | String | Failure phase (init / ocr) |
| `country`          | String | Country code if inferred   |
| `created_at`       | String | RFC3339 timestamp          |

## Required IAM Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:HeadBucket",
        "s3:CreateBucket",
        "s3:GetObject",
        "s3:PutObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::identity-card-ocr",
        "arn:aws:s3:::identity-card-ocr/*"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:DescribeTable",
        "dynamodb:CreateTable",
        "dynamodb:PutItem",
        "dynamodb:GetItem"
      ],
      "Resource": ["arn:aws:dynamodb:ap-east-1:*:table/identity-card-ocr-*"]
    },
    {
      "Effect": "Allow",
      "Action": [
        "events:DescribeEventBus",
        "events:CreateEventBus",
        "events:PutEvents"
      ],
      "Resource": "arn:aws:events:ap-east-1:*:event-bus/identity-card-ocr-bus"
    },
    {
      "Effect": "Allow",
      "Action": "sts:GetCallerIdentity",
      "Resource": "*"
    }
  ]
}
```

## Deploying to AWS Lambda

1. Build the binary:

   ```bash
   GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build -o bootstrap ./cmd/lambda/main.go
   ```

2. Create a Lambda function:

   - Runtime: `provided.al2023`
   - Architecture: `arm64`
   - Timeout: **≥ 60 seconds** (DynamoDB table creation takes 15–30s on first deploy)
   - Memory: **≥ 512 MB** (Tesseract OCR requires significant memory)

3. Configure S3 trigger on the bucket with suffix `.png` or `.jpg`

4. Set environment variables:
   - `IS_PRODUCTION=true`
   - `AWS_CONFIG_PATH` (optional, defaults to `aws-config.yml` in the deployment package)
   - `LOG_LEVEL` (optional, defaults to `INFO`)

## License

MIT License. Copyright (c) 2026 ctkqiang.

## Documentation

Full documentation in English and Chinese:

| Document | EN | ZH |
|----------|----|----|
| Index | [docs/index_en.md](docs/index_en.md) | [docs/index_zh.md](docs/index_zh.md) |
| Architecture | [docs/architecture_en.md](docs/architecture_en.md) | [docs/architecture_zh.md](docs/architecture_zh.md) |
| Lambda Flow | [docs/lambda-flow_en.md](docs/lambda-flow_en.md) | [docs/lambda-flow_zh.md](docs/lambda-flow_zh.md) |
| OCR Pipeline | [docs/ocr-pipeline_en.md](docs/ocr-pipeline_en.md) | [docs/ocr-pipeline_zh.md](docs/ocr-pipeline_zh.md) |
| Infrastructure | [docs/infrastructure_en.md](docs/infrastructure_en.md) | [docs/infrastructure_zh.md](docs/infrastructure_zh.md) |
| Configuration | [docs/configuration_en.md](docs/configuration_en.md) | [docs/configuration_zh.md](docs/configuration_zh.md) |
| Development | [docs/development_en.md](docs/development_en.md) | [docs/development_zh.md](docs/development_zh.md) |
| Deployment | [docs/deployment_en.md](docs/deployment_en.md) | [docs/deployment_zh.md](docs/deployment_zh.md) |
