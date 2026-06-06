# 基础设施

## 自动配置

每次冷启动时，`EnsureInfrastructure()` 验证并创建缺失的 AWS 资源。所有检查是幂等的——已存在的资源会被检测到并跳过，无副作用。

### 资源生命周期

```
EnsureInfrastructure(ctx)
  │
  ├── EnsureS3Bucket(bucket, region)
  │     ├── HeadBucket  → 存在？返回
  │     ├── CreateBucket（带 LocationConstraint）
  │     └── 等待桶可访问（30 秒超时）
  │
  ├── EnsureDynamoDBTable("identity-card-ocr-users", "document_id")
  │     ├── DescribeTable → ACTIVE？返回
  │     ├── CreateTable（PAY_PER_REQUEST）
  │     └── 等待 ACTIVE（2 分钟超时）
  │
  ├── EnsureDynamoDBTable("identity-card-ocr-failed", "document_id")
  │     └── （同上）
  │
  └── EnsureEventBridgeBus("identity-card-ocr-bus")
        ├── DescribeEventBus → 存在？返回
        └── CreateEventBus
```

### DynamoDB 表结构

**identity-card-ocr-users**（通过的文档）

| 属性 | 类型 | 键 |
|-----------|------|-----|
| `document_id` | String (S) | HASH (主键) |

计费模式: `PAY_PER_REQUEST`

**identity-card-ocr-failed**（失败的尝试）

| 属性 | 类型 | 键 |
|-----------|------|-----|
| `document_id` | String (S) | HASH (主键) |

计费模式: `PAY_PER_REQUEST`

## 所需 IAM 权限

Lambda 执行角色必须具备以下权限。详情参见 [infrastructure_en.md](infrastructure_en.md)。

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

## 冷启动延迟

| 场景 | API 调用次数 | 耗时 |
|----------|-----------|----------|
| 所有资源已存在 | 4（HeadBucket、2×DescribeTable、DescribeEventBus） | ~500ms |
| 创建 DynamoDB 表 | 6（含 2×CreateTable + 等待） | 15–30 秒 |
| 首次部署（全部资源） | 8+（含桶 + 总线创建） | 20–45 秒 |

后续热调用完全跳过 `main()`，零基础设施开销。
