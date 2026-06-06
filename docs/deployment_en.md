# Deployment Guide

## Building for AWS Lambda

### Binary Build

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build -o bootstrap ./cmd/lambda/main.go
```

The binary must be named `bootstrap` for the `provided.al2023` Lambda runtime. CGo must be enabled because gosseract links against the Tesseract C library.

### Lambda Configuration

| Setting | Value | Notes |
|---------|-------|-------|
| Runtime | `provided.al2023` | Custom runtime (Go binary) |
| Architecture | `arm64` (Graviton) | Lower cost, better performance |
| Memory | 512 MB minimum | Tesseract OCR requires significant memory for image processing |
| Timeout | 60 seconds minimum | DynamoDB table creation on first deploy takes 15–30s |
| Ephemeral storage | 512 MB (default) | Images are downloaded to `/tmp` and deleted after processing |

### Environment Variables

Set on the Lambda function:

```
IS_PRODUCTION=true
LOG_LEVEL=INFO
```

Credentials are loaded from the Lambda IAM role — do not set `AWS_ACCESS_KEY_ID` or `AWS_SECRET_ACCESS_KEY` in production.

### S3 Trigger Configuration

Create an S3 trigger on the bucket with:
- Event type: `s3:ObjectCreated:*`
- Suffix filter: `.png`, `.jpg`, `.jpeg`

### IAM Role

Attach a policy granting the permissions documented in [infrastructure_en.md](infrastructure_en.md).

## Deployment Package

The Lambda deployment package requires the Tesseract shared library:

```bash
# Build with Tesseract statically linked (recommended for simpler deployment)
# Or bundle the .so file in the deployment package
```

For production deployments, use a Docker-based Lambda container image:

```dockerfile
FROM public.ecr.aws/lambda/provided:al2023

RUN dnf install -y tesseract tesseract-langpack-chi-sim tesseract-langpack-eng

COPY bootstrap /var/runtime/
```

## Post-Deployment Verification

1. Upload a test image to the S3 bucket: `china/test-id-card.png`
2. Check CloudWatch Logs for the Lambda invocation
3. Verify the result in DynamoDB `identity-card-ocr-users` table
4. Confirm EventBridge events are published to the event bus
