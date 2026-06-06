# Configuration Reference

## aws-config.yml

Primary configuration file for AWS resource names and region. Located in the working directory or at a path set by `AWS_CONFIG_PATH`.

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

### Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `environment.region` | string | `"ap-east-1"` | AWS region for all resources |
| `environment.profile` | string | `"default"` | AWS credentials profile name (local dev only) |
| `environment.s3.bucket` | string | `"identity-card-ocr"` | S3 bucket name for image uploads and event store |
| `environment.s3.path` | string | `"identity/"` | S3 key prefix for all objects |
| `environment.eventbridge.bus_name` | string | `""` (default bus) | Custom event bus name; empty = default bus |
| `environment.eventbridge.source` | string | `"identity-card-ocr"` | EventBridge event Source field value |
| `environment.dynamodb.user_identity_table` | string | `"identity-card-ocr-users"` | Table for successfully processed documents |
| `environment.dynamodb.failed_records_table` | string | `"identity-card-ocr-failed"` | Table for failed OCR attempts |

### Fallback Behavior

When `aws-config.yml` is missing, malformed, or contains empty fields, the application falls back to the documented defaults above. The application starts and runs normally with defaults; no error is returned.

## Environment Variables

### .env File

```
IS_PRODUCTION=true
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=...
AWS_REGION=ap-east-1
```

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `IS_PRODUCTION` | No | `false` | Set to `"true"` to run in Lambda mode |
| `AWS_ACCESS_KEY_ID` | No | SDK chain | AWS access key (highest priority credential source) |
| `AWS_SECRET_ACCESS_KEY` | No | SDK chain | AWS secret key |
| `AWS_REGION` | No | `ap-east-1` | AWS region override |
| `LOG_LEVEL` | No | `INFO` | Log level: DEBUG, INFO, WARN, ERROR, VVERBOSE |
| `AWS_CONFIG_PATH` | No | `./aws-config.yml` | Path to aws-config.yml |
| `DOTENV_PATH` | No | `./.env` | Path to .env file |

### Credential Resolution Order

1. `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` environment variables
2. `.env` file in the working directory
3. AWS SDK default credential chain:
   - `~/.aws/credentials`
   - `AWS_PROFILE` environment variable
   - IAM instance profile (EC2) / IAM task role (ECS/Lambda)
