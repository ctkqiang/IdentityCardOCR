# 架构设计

## 系统架构

```
                         ┌──────────────────────────┐
                         │      AWS Account           │
                         │                           │
  ┌──────────┐           │  ┌─────────────────────┐  │
  │  客户端   │──上传──┼─▶│    S3 桶              │  │
  │ (手机/  │  图片    │  │  identity-card-ocr   │  │
  │  Web)    │           │  │  /identity/*.png     │  │
  └──────────┘           │  └──────────┬──────────┘  │
                         │             │ S3 事件     │
                         │             ▼             │
                         │  ┌─────────────────────┐  │
                         │  │   AWS Lambda         │  │
                         │  │   (Go + Tesseract)   │  │
                         │  │                      │  │
                         │  │  处理器接收 S3 事件，  │  │
                         │  │  下载图片、运行 OCR、  │  │
                         │  │  解析字段并按国家标准  │  │
                         │  │  验证数据             │  │
                         │  └────┬──────────┬─────┘  │
                         │       │          │         │
                         │       ▼          ▼         │
                         │  ┌─────────┐ ┌──────────┐ │
                         │  │DynamoDB │ │EventBridge│ │
                         │  │(用户表) │ │  (事件总线) │ │
                         │  │(失败表) │ │          │ │
                         │  └─────────┘ └────┬─────┘ │
                         │                    │        │
                         │                    ▼        │
                         │  ┌──────────────────────┐  │
                         │  │  下游系统              │  │
                         │  │  (审计、分析、通知)    │  │
                         │  └──────────────────────┘  │
                         └──────────────────────────┘
```

## 包依赖关系

```
main.go
  ├── internal/config           (YAML + 环境变量 配置加载器)
  ├── internal/lambda           (S3 事件处理器)
  │     ├── internal/config
  │     ├── internal/event      (事件类型 + 存储 + 总线)
  │     ├── internal/pipeline   (事件驱动 OCR 管道)
  │     ├── internal/service    (OCR + 解析器)
  │     ├── internal/service/aws       (认证 + 基础设施)
  │     └── internal/service/dynamodb  (数据访问)
  ├── internal/service/aws
  │     ├── internal/config
  │     └── internal/utilities
  ├── internal/service/dynamodb   (独立；仅依赖 AWS SDK)
  └── internal/utilities          (独立；无内部依赖)
```

## 设计决策

| 决策 | 理由 |
|----------|-----------|
| **AWS SDK v2** | 最新 SDK，性能更好，支持 middleware 和 context 传播 |
| **基于 STS 的单例认证** | 全局共享一个已验证的 SDK 配置，避免重复加载凭证 |
| **PAY_PER_REQUEST 计费** | 无需容量规划，从零到任意吞吐量自动扩展；适合波动负载 |
| **S3 事件存储** | 仅追加的不可变日志；当 EventBridge 投递失败时作为持久化事实来源 |
| **gosseract v2 (CGo)** | 直接调用 Tesseract C API；比子进程方式更快；支持 PSM 和白名单配置 |
| **无框架** | 纯 Go，最小依赖；无 DI 容器、无 ORM、无代码生成 |
| **幂等基础设施** | 每次冷启动验证基础设施；可安全重复执行；无需外部 IaC 工具 |

## 错误处理

每个失败路径：
1. 发送 `processing.failed` 事件到 S3 事件存储（持久化）
2. 发布相同事件到 EventBridge（实时通知）
3. 写入 `FailedRecord` 到 DynamoDB `failed_records` 表
4. 继续处理下一个 S3 对象（永不中止整个批次）

失败阶段标签：
- `"init"`：国家推断或 S3 下载失败
- `"ocr"`：Tesseract OCR 或字段提取失败
