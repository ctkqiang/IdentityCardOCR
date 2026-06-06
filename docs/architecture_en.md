# Architecture

## System Architecture

```
                         ┌──────────────────────────┐
                         │      AWS Account           │
                         │                           │
  ┌──────────┐           │  ┌─────────────────────┐  │
  │  Client   │──Upload──┼─▶│    S3 Bucket         │  │
  │ (Mobile/ │  image    │  │  identity-card-ocr   │  │
  │  Web)    │           │  │  /identity/*.png     │  │
  └──────────┘           │  └──────────┬──────────┘  │
                         │             │ S3 Event    │
                         │             ▼             │
                         │  ┌─────────────────────┐  │
                         │  │   AWS Lambda         │  │
                         │  │   (Go + Tesseract)   │  │
                         │  │                      │  │
                         │  │  Handler receives    │  │
                         │  │  S3 event, downloads │  │
                         │  │  image, runs OCR,    │  │
                         │  │  parses fields, and  │  │
                         │  │  validates against   │  │
                         │  │  country standards   │  │
                         │  └────┬──────────┬─────┘  │
                         │       │          │         │
                         │       ▼          ▼         │
                         │  ┌─────────┐ ┌──────────┐ │
                         │  │DynamoDB │ │EventBridge│ │
                         │  │(users)  │ │  (bus)   │ │
                         │  │(failed) │ │          │ │
                         │  └─────────┘ └────┬─────┘ │
                         │                    │        │
                         │                    ▼        │
                         │  ┌──────────────────────┐  │
                         │  │  Downstream Systems   │  │
                         │  │  (audit, analytics,   │  │
                         │  │   notifications)      │  │
                         │  └──────────────────────┘  │
                         └──────────────────────────┘
```

## Package Dependency Graph

```
main.go
  ├── internal/config           (YAML + env config loader)
  ├── internal/lambda           (S3 event handler)
  │     ├── internal/config
  │     ├── internal/event      (event types + store + bridge)
  │     ├── internal/pipeline   (event-driven OCR pipeline)
  │     ├── internal/service    (OCR + parser)
  │     ├── internal/service/aws       (auth + infra)
  │     └── internal/service/dynamodb  (data access)
  ├── internal/service/aws
  │     ├── internal/config
  │     └── internal/utilities
  ├── internal/service/dynamodb   (standalone; AWS SDK only)
  └── internal/utilities          (standalone; no internal deps)
```

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **AWS SDK v2** | Latest SDK with improved performance, middleware support, and native context propagation |
| **Singleton auth via STS** | One verified SDK config shared across all service clients; no redundant credential resolution |
| **PAY_PER_REQUEST DynamoDB** | Zero capacity planning; scales from 0 to any throughput; cost-efficient for variable workloads |
| **S3-backed event store** | Append-only immutable log; functions as durable source of truth when EventBridge delivery fails |
| **gosseract v2 (CGo)** | Direct Tesseract C API binding; faster than subprocess-based approaches; supports PSM and whitelist |
| **No framework** | Plain Go with minimal dependencies; no DI containers, ORMs, or code generation |
| **Idempotent infrastructure** | Infrastructure verified on every cold start; safe to repeat; no external IaC tool required |

## Error Handling

Every failure path:
1. Emits `processing.failed` event to the S3 event store (durable)
2. Publishes the same event to EventBridge (real-time notification)
3. Writes a `FailedRecord` to DynamoDB `failed_records` table
4. Continues to the next S3 object (never aborts the entire batch)

Failure phase labels:
- `"init"`: Country inference or S3 download failure
- `"ocr"`: Tesseract OCR or field extraction failure
