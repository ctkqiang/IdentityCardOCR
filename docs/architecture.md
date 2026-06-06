# Architecture / 架构设计

[English](#english) | [中文](#中文)

---

## English

### System Architecture

```
                         ┌──────────────────────────┐
                         │      AWS Account          │
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
                         │  │  1. Download image   │  │
                         │  │  2. Tesseract OCR    │  │
                         │  │  3. Parse fields     │  │
                         │  │  4. Validate (GB/My) │  │
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

### Component Diagram

```
main.go
  ├── [Production]  lambda.Start(HandleRequest)
  │     ├── aws.Init()                    → STS auth singleton
  │     └── aws.EnsureInfrastructure()    → S3 + DynamoDB + EventBridge
  │
  └── [Development] runDevCLI()
        └── service.ExtractTextFromIdentityDocument()
              ├── GetTesseractClient()
              │     ├── SetImage()
              │     ├── SetLanguage()     (chi_sim / eng)
              │     └── Text()
              └── ParseOCR()
                    ├── parseChinaIDCard()
                    │     ├── extractIDNumber()
                    │     ├── ParseIDInfo()     (GB11643-1999)
                    │     ├── extractChineseName()
                    │     └── extractExpiryDate()
                    └── parseMalaysiaMyKad()
                          ├── MyKadDOB()
                          ├── MyKadSex()
                          ├── extractMyKadName()
                          └── extractMyKadAddress()
```

### Package Dependency Graph

```
main.go
  ├── internal/config           (YAML + env config loader)
  ├── internal/lambda           (S3 event handler)
  │     ├── internal/config
  │     ├── internal/event      ◀── event types + store + bridge
  │     ├── internal/pipeline   ◀── event-driven OCR pipeline
  │     ├── internal/service    ◀── OCR + parser
  │     ├── internal/service/aws       ◀── auth + infra
  │     └── internal/service/dynamodb  ◀── data access
  ├── internal/service/aws
  │     ├── internal/config
  │     └── internal/utilities
  ├── internal/service/dynamodb   (standalone, depends only on AWS SDK)
  └── internal/utilities          (standalone, no internal deps)
```

### Design Decisions

| Decision | Rationale |
|----------|-----------|
| **AWS SDK v2** | Latest SDK with improved performance, middleware, and context support |
| **Singleton auth** | One STS-verified config shared across all service clients; no redundant credential loading |
| **PAY_PER_REQUEST billing** | Zero capacity planning; scales from 0 to any throughput; cost-efficient for variable workloads |
| **S3 event store** | Append-only immutable log; durable source of truth even if EventBridge delivery fails |
| **gosseract v2 (CGo)** | Direct Tesseract C API binding; faster than subprocess; supports PSM and whitelist config |
| **No framework** | Plain Go with minimal dependencies; no DI containers, no ORMs, no code generation |
| **Idempotent infra** | Infrastructure checks on every cold start; safe to run repeatedly; no external IaC needed |

### Error Handling Strategy

```
                    ┌─────────────┐
                    │  S3 Event    │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ inferCountry│
                    └──┬──────┬──┘
                       │      └──(fail)──▶ emitFailure("init")
                       │
                ┌──────▼──────┐
                │ download S3 │
                └──┬──────┬──┘
                   │      └──(fail)──▶ emitFailure("init")
                   │
            ┌──────▼──────┐
            │ Tesseract   │
            │ OCR + Parse │
            └──┬──────┬──┘
               │      └──(fail)──▶ emitFailure("ocr")
               │
        ┌──────▼──────┐
        │ DynamoDB    │
        │ PutIdentity │──▶ SUCCESS
        │ + Event     │
        └─────────────┘
```

Every failure path:
1. Emits a `processing.failed` event to S3 (durable) and EventBridge (notification)
2. Writes a `FailedRecord` to DynamoDB `failed_records` table
3. Continues processing remaining S3 objects (never fails the entire batch)

---

## 中文

### 系统架构

用户通过移动端或网页上传证件图片到 S3 → S3 事件触发 Lambda → Lambda 下载图片、运行 Tesseract OCR、解析字段、验证（中国 GB11643-1999 标准 / 马来西亚 MyKad 规则）→ 成功结果存入 DynamoDB `user_identity` 表 + 发送 `processing.completed` 事件 → 失败结果存入 `failed_records` 表 + 发送 `processing.failed` 事件 → 下游系统通过 EventBridge 消费事件。

### 组件图

参见上方英文部分的组件图。

### 包依赖关系

参见上方英文部分的依赖图。

### 设计决策

| 决策 | 理由 |
|----------|-----------|
| **AWS SDK v2** | 最新 SDK，性能更好，支持 middleware 和 context |
| **单例认证** | 全局共享一个 STS 验证过的配置，避免重复加载凭证 |
| **PAY_PER_REQUEST 计费** | 无需容量规划，从零到任意吞吐量自动扩展 |
| **S3 事件存储** | 仅追加的不可变日志；即使 EventBridge 投递失败也有持久化记录 |
| **gosseract v2 (CGo)** | 直接调用 Tesseract C API；比子进程方式更快；支持 PSM 和白名单配置 |
| **无框架** | 纯 Go，最小依赖；无 DI 容器、无 ORM、无代码生成 |
| **幂等基础设施** | 每次冷启动检查基础设施；可安全重复执行；无需外部 IaC 工具 |

### 错误处理策略

参见上方英文部分的错误处理流程图。每个失败路径都会：
1. 发送 `processing.failed` 事件到 S3（持久）和 EventBridge（通知）
2. 写入 `FailedRecord` 到 DynamoDB `failed_records` 表
3. 继续处理剩余的 S3 对象（永不让整个批次失败）
