# IdentityCardOCR

![](./docs/logo.svg)

[English](README.md) | [中文](README_ZH.md)

生产级、开源的 AWS Lambda 服务，基于 OCR 实现身份证件信息提取。支持中华人民共和国居民身份证和马来西亚 MyKad/MyPR 证件，具备自动 AWS 基础设施配置能力。

## 架构概览

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   S3 桶        │────▶│ AWS Lambda   │────▶│  DynamoDB    │
│ (图片上传)     │     │ (OCR 引擎)   │     │ (结构化数据)  │
└──────────────┘     └──────┬───────┘     └──────────────┘
                            │
                            ▼
                     ┌──────────────┐
                     │ EventBridge  │
                     │ (事件总线)    │
                     └──────┬───────┘
                            │
                            ▼
                     ┌──────────────┐
                     │  下游消费者   │
                     └──────────────┘
```

**流程：** 客户端上传图片到 S3 → S3 事件触发 Lambda → Tesseract OCR 提取文字 → 解析器提取结构化字段 → 通过的结果存入 DynamoDB `user_identity` 表 + 发送 `processing.completed` 事件 → 失败的结果存入 DynamoDB `failed_records` 表 + 发送 `processing.failed` 事件。

## 支持的证件

| 国家 | 证件类型 | OCR 语言 | 解析方式 |
|------|---------|---------|---------|
| 中国 (`china`) | 居民身份证 | `chi_sim` | GB11643-1999 校验和验证 |
| 中国 (`china`) | 中国护照 | `chi_sim` | 原始文本提取 |
| 马来西亚 (`malaysia`) | MyKad / MyPR | `eng` | 12 位数字模式 + 出生日期/性别推导 |
| 马来西亚 (`malaysia`) | 马来西亚护照 | `eng` | 原始文本提取 |

## 项目结构

```
IdentityCardOCR/
├── main.go                          # 入口文件（生产 Lambda / 开发 CLI 分发）
├── aws-config.yml                   # AWS 资源配置（区域、桶、表）
├── .env                             # AWS 凭证（git 忽略）
├── .env.example                     # 凭证模板
├── Makefile                         # 构建目标（test, build, clean）
├── go.mod / go.sum                  # Go 模块依赖
│
├── cmd/
│   └── lambda/
│       └── main.go                  # 独立 Lambda 二进制入口
│
├── internal/
│   ├── config/
│   │   └── config.go                # YAML + .env 配置加载器
│   │
│   ├── model/
│   │   ├── document_info.go         # DocumentInfo、ChineseIdentityCardInfo 类型
│   │   ├── pipeline.go              # OCR 管道结构体（S3 + Textract 客户端）
│   │   └── response.go              # Lambda 响应信封
│   │
│   ├── event/
│   │   ├── types.go                 # 事件信封、负载类型、UUID 生成
│   │   ├── store.go                 # 基于 S3 的仅追加事件日志
│   │   ├── bridge.go                # EventBridge 发布（单个 + 批量）
│   │   └── event.go                 # LambdaEvent 类型
│   │
│   ├── pipeline/
│   │   └── pipeline.go              # 事件驱动 OCR 管道编排器
│   │
│   ├── lambda/
│   │   └── handler.go               # 主 Lambda S3 事件处理器
│   │
│   ├── service/
│   │   ├── aws.go                   # AWS 服务类型常量 + SDK 配置委托
│   │   ├── tesseract.go             # Tesseract OCR 集成（gosseract v2）
│   │   ├── parser.go                # OCR 文本 → 结构化 DocumentInfo 解析器
│   │   │
│   │   ├── aws/
│   │   │   ├── account.go           # AWS 认证单例（STS 验证）
│   │   │   ├── infra.go             # 基础设施自动配置
│   │   │   ├── s3.go                # S3 客户端工厂
│   │   │   └── lambda.go            # 包文档（处理器已移至 internal/lambda/）
│   │   │
│   │   └── dynamodb/
│   │       └── client.go            # DynamoDB 客户端（PutUserIdentity、PutFailedRecord、GetUserIdentity）
│   │
│   └── utilities/
│       ├── logger.go                # 结构化 CloudWatch 日志框架
│       └── validator.go             # 中国身份证 + MyKad 验证逻辑
│
├── sample/
│   ├── china/
│   │   ├── identity_card.png        # 示例中国身份证
│   │   └── passport.png             # 示例中国护照
│   └── malaysia/
│       ├── identity_card.png        # 示例马来西亚 MyKad
│       └── passport.png             # 示例马来西亚护照
│
├── test/
│   └── id_test.go                   # OCR + 解析器集成测试
│
└── docs/                            # 完整文档
```

## 快速开始

### 前置条件

- Go 1.26+
- Tesseract OCR 5.x（包含语言数据 `chi_sim`、`eng`）
- 已配置凭证的 AWS 账户

### 本地开发（开发模式）

```bash
# 安装 Tesseract（macOS）
brew install tesseract tesseract-lang

# 确保 .env 中 IS_PRODUCTION 未设置为 "true"
# 以开发模式运行 — 交互式 CLI 扫描器
go run main.go
```

开发模式为交互式 CLI：提供图片路径和国家代码，工具运行 OCR 并将提取的身份字段打印到终端。无需 AWS 资源。

### 运行测试

```bash
make test
```

### 生产构建

```bash
# 设置生产模式
export IS_PRODUCTION=true

# 构建 ARM64 Lambda 二进制
GOOS=linux GOARCH=arm64 go build -o bootstrap ./cmd/lambda/main.go
```

## 配置

所有 AWS 资源名称和区域在 `aws-config.yml` 中定义：

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

凭证从 `.env`（最高优先级）或 AWS SDK 默认凭证链（Lambda 使用 IAM 角色，本地开发使用 `~/.aws/credentials`）加载。

## 基础设施自动配置

冷启动时，应用程序自动验证并创建缺失的 AWS 资源：

1. **S3 桶** — HeadBucket → CreateBucket（带区域特定的 LocationConstraint）
2. **DynamoDB 表** — DescribeTable → 使用 `PAY_PER_REQUEST` 计费创建表 → 等待 ACTIVE
3. **EventBridge 总线** — DescribeEventBus → CreateEventBus

所有检查是幂等的 — 可安全地在每次冷启动时运行，无副作用。

## 事件系统

应用程序使用事件驱动架构，包含三种事件类型：

| 事件类型 | 详情负载 | 触发时机 |
|---------|---------|---------|
| `document.submitted` | `DocumentSubmittedPayload`（image_path、s3_bucket、s3_key、country） | 客户端提交文档时 |
| `processing.completed` | `ProcessingCompletedPayload`（id_number、name、nationality、dob、sex、expiry_date、raw_text） | OCR + 解析成功时 |
| `processing.failed` | `ProcessingFailedPayload`（error、phase） | 任何处理步骤失败时 |

事件持久存储在 S3（`{prefix}/events/{documentID}/{timestamp}.json`）并发布到 EventBridge 供下游消费者使用。

## DynamoDB 数据模型

### `identity-card-ocr-users`（通过）

| 字段 | 类型 | 描述 |
|------|------|------|
| `document_id`（主键） | String | S3 对象键 |
| `id_number` | String | 提取的证件号码 |
| `name` | String | 持卡人姓名 |
| `date_of_birth` | String | YYYY-MM-DD |
| `sex` | String | 男/女 或 LELAKI/PEREMPUAN |
| `nationality` | String | 地区或国籍 |
| `expiry_date` | String | 证件有效期（如找到） |
| `raw_text` | String | 原始 OCR 输出 |
| `country` | String | china / malaysia / us |
| `created_at` | String | RFC3339 时间戳 |

### `identity-card-ocr-failed`（失败）

| 字段 | 类型 | 描述 |
|------|------|------|
| `document_id`（主键） | String | S3 对象键 |
| `error` | String | 错误信息 |
| `phase` | String | 失败阶段（init / ocr） |
| `country` | String | 国家代码（如能推断） |
| `created_at` | String | RFC3339 时间戳 |

## 所需 IAM 权限

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

## 部署到 AWS Lambda

1. 构建二进制文件：

   ```bash
   GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build -o bootstrap ./cmd/lambda/main.go
   ```

2. 创建 Lambda 函数：

   - 运行时: `provided.al2023`
   - 架构: `arm64`
   - 超时: **≥ 60 秒**（首次部署时 DynamoDB 表创建需要 15–30 秒）
   - 内存: **≥ 512 MB**（Tesseract OCR 需要较多内存）

3. 在 S3 桶上配置触发器（后缀 `.png` 或 `.jpg`）

4. 设置环境变量：
   - `IS_PRODUCTION=true`
   - `AWS_CONFIG_PATH`（可选，默认为部署包中的 `aws-config.yml`）
   - `LOG_LEVEL`（可选，默认为 `INFO`）

## 许可证

MIT License. Copyright (c) 2026 ctkqiang.

## 文档

完整的中英文文档：

| 文档 | EN | ZH |
|------|----|----|
| 索引 | [docs/index_en.md](docs/index_en.md) | [docs/index_zh.md](docs/index_zh.md) |
| 架构设计 | [docs/architecture_en.md](docs/architecture_en.md) | [docs/architecture_zh.md](docs/architecture_zh.md) |
| Lambda 流程 | [docs/lambda-flow_en.md](docs/lambda-flow_en.md) | [docs/lambda-flow_zh.md](docs/lambda-flow_zh.md) |
| OCR 管道 | [docs/ocr-pipeline_en.md](docs/ocr-pipeline_en.md) | [docs/ocr-pipeline_zh.md](docs/ocr-pipeline_zh.md) |
| 基础设施 | [docs/infrastructure_en.md](docs/infrastructure_en.md) | [docs/infrastructure_zh.md](docs/infrastructure_zh.md) |
| 配置参考 | [docs/configuration_en.md](docs/configuration_en.md) | [docs/configuration_zh.md](docs/configuration_zh.md) |
| 开发指南 | [docs/development_en.md](docs/development_en.md) | [docs/development_zh.md](docs/development_zh.md) |
| 部署指南 | [docs/deployment_en.md](docs/deployment_en.md) | [docs/deployment_zh.md](docs/deployment_zh.md) |
