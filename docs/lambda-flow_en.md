# Lambda Processing Flow

## Entry Point

The Lambda function is triggered by S3 `ObjectCreated:*` events. Each invocation receives an `events.S3Event` containing one or more S3 object records.

## Handler Lifecycle

### Cold Start (main.go)

```
aws.Init()                   → STS authentication, cache SDK config
aws.EnsureInfrastructure()   → verify/create S3 bucket, DynamoDB tables, EventBridge bus
lambda.Start(HandleRequest)  → enter Lambda runtime loop (blocks forever)
```

### Per-Invocation (HandleRequest)

For each S3 object in the event:

```
1. inferCountry(key)
   └─ Parse "china/..." or "malaysia/..." prefix
   └─ Failure → emitFailure("init"), continue to next object

2. downloadS3Object(bucket, key, /tmp/{filename})
   └─ Failure → emitFailure("init"), continue to next object

3. service.ExtractTextFromIdentityDocument(/tmp/{filename}, country)
   ├─ GetTesseractClient()
   ├─ SetImage(path)
   ├─ SetLanguage(chi_sim / eng)
   ├─ Text() → raw OCR string
   ├─ ParseOCR(rawText, country)
   │   ├─ [China]  parseChinaIDCard()
   │   │   ├─ extractIDNumber()      → 18-digit regex
   │   │   ├─ ParseIDInfo()          → GB11643-1999 validation
   │   │   ├─ extractChineseName()   → heuristic name extraction
   │   │   └─ extractExpiryDate()    → date range extraction
   │   └─ [Malaysia] parseMalaysiaMyKad()
   │       ├─ MyKadDOB()             → YYMMDD → YYYY-MM-DD
   │       ├─ MyKadSex()             → last digit parity
   │       ├─ extractMyKadName()     → uppercase word heuristic
   │       └─ extractMyKadAddress()  → post-ID address block
   └─ Return DocumentInfo

4. [Success]
   ├─ pipe.Process(processing.completed event)
   │   ├─ store.Append()      → S3 {prefix}/events/{docID}/{ts}.json
   │   └─ bus.Put()           → EventBridge
   └─ ddb.PutUserIdentity()   → DynamoDB user_identity table

5. [Failure]
   ├─ pipe.Process(processing.failed event)
   │   ├─ store.Append()      → S3 event log
   │   └─ bus.Put()           → EventBridge
   └─ ddb.PutFailedRecord()   → DynamoDB failed_records table

6. os.Remove(/tmp/{filename})

7. Continue to next S3 object
```

## Response Format

```json
{
  "message": "processed 3 objects",
  "total": 3,
  "passed": 2,
  "failed": 1
}
```

## Country Inference

The document country is inferred from the S3 object key prefix:

| Key Prefix | Country |
|-----------|---------|
| `china/...` | China (chi_sim OCR, GB11643-1999 validation) |
| `malaysia/...` | Malaysia (eng OCR, MyKad rules) |
| `us/...` | US (eng OCR, no specific parser) |
| Unknown prefix | Failure (`"init"` phase) |

## Concurrency

Each S3 event triggers one Lambda invocation. Multiple S3 objects in the same event are processed sequentially within that invocation. Concurrent uploads trigger separate Lambda invocations that run in parallel.

## Temporary Files

Images are downloaded to `/tmp` (the Lambda ephemeral storage). Files are deleted immediately after OCR processing to avoid exceeding the 512 MB storage limit on subsequent invocations.
