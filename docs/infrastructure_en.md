# Infrastructure

## Auto-Provisioning

On every cold start, `EnsureInfrastructure()` verifies and creates missing AWS resources. All checks are idempotent — resources that already exist are detected and skipped without side effects.

### Resource Lifecycle

```
EnsureInfrastructure(ctx)
  │
  ├── EnsureS3Bucket(bucket, region)
  │     ├── HeadBucket  → exists? return
  │     ├── CreateBucket (with LocationConstraint)
  │     └── Wait for bucket to exist (30s timeout)
  │
  ├── EnsureDynamoDBTable("identity-card-ocr-users", "document_id")
  │     ├── DescribeTable → ACTIVE? return
  │     ├── CreateTable (PAY_PER_REQUEST)
  │     └── Wait for ACTIVE (2 min timeout)
  │
  ├── EnsureDynamoDBTable("identity-card-ocr-failed", "document_id")
  │     └── (same as above)
  │
  └── EnsureEventBridgeBus("identity-card-ocr-bus")
        ├── DescribeEventBus → exists? return
        └── CreateEventBus
```

### DynamoDB Table Schemas

**identity-card-ocr-users** (passed documents)

| Attribute | Type | Key |
|-----------|------|-----|
| `document_id` | String (S) | HASH (PK) |

Billing mode: `PAY_PER_REQUEST`

**identity-card-ocr-failed** (failed attempts)

| Attribute | Type | Key |
|-----------|------|-----|
| `document_id` | String (S) | HASH (PK) |

Billing mode: `PAY_PER_REQUEST`

## Required IAM Permissions

The Lambda execution role must have the following permissions.

### S3

```json
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
}
```

### DynamoDB

```json
{
  "Effect": "Allow",
  "Action": [
    "dynamodb:DescribeTable",
    "dynamodb:CreateTable",
    "dynamodb:PutItem",
    "dynamodb:GetItem"
  ],
  "Resource": "arn:aws:dynamodb:ap-east-1:*:table/identity-card-ocr-*"
}
```

### EventBridge

```json
{
  "Effect": "Allow",
  "Action": [
    "events:DescribeEventBus",
    "events:CreateEventBus",
    "events:PutEvents"
  ],
  "Resource": "arn:aws:events:ap-east-1:*:event-bus/identity-card-ocr-bus"
}
```

### STS

```json
{
  "Effect": "Allow",
  "Action": "sts:GetCallerIdentity",
  "Resource": "*"
}
```

## Cold Start Latency

| Scenario | API Calls | Duration |
|----------|-----------|----------|
| All resources exist | 4 (HeadBucket, 2×DescribeTable, DescribeEventBus) | ~500ms |
| Creating DynamoDB tables | 6 (includes 2×CreateTable + waiters) | 15–30s |
| First deploy (all resources) | 8+ (includes bucket + bus creation) | 20–45s |

Subsequent warm invocations skip `main()` entirely and pay zero infrastructure overhead.
