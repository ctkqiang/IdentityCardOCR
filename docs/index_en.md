# IdentityCardOCR — Documentation

Open-source, production-grade AWS Lambda service for OCR-based identity document extraction. Supports Chinese Resident Identity Cards (中华人民共和国居民身份证) and Malaysian MyKad documents.

## Table of Contents

| Document | Description |
|----------|-------------|
| [Architecture](architecture_en.md) | System architecture, component relationships, and design decisions |
| [Lambda Flow](lambda-flow_en.md) | S3 event to OCR to DynamoDB processing pipeline in detail |
| [OCR Pipeline](ocr-pipeline_en.md) | Tesseract integration, text parsing, and country-specific extractors |
| [Infrastructure](infrastructure_en.md) | S3, DynamoDB, EventBridge auto-provisioning and IAM permissions |
| [Configuration](configuration_en.md) | aws-config.yml, .env, and environment variable reference |
| [Development](development_en.md) | Local development setup, testing, and CLI dev mode |
| [Deployment](deployment_en.md) | Building and deploying to AWS Lambda |

## Quick Links

- Project logo: [logo.svg](logo.svg)
- Chinese documentation: [index_zh.md](index_zh.md)
