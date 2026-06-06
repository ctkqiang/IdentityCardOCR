# IdentityCardOCR — Documentation

[English](#english) | [中文](#中文)

---

## English

IdentityCardOCR is an open-source, production-grade AWS Lambda service for OCR-based identity document extraction. It supports Chinese Resident Identity Cards and Malaysian MyKad documents with automatic infrastructure provisioning.

### Documentation Index

| Document | Description |
|----------|-------------|
| [Architecture](architecture.md) | System architecture, component relationships, and design decisions |
| [Lambda Flow](lambda-flow.md) | S3 event → OCR → DynamoDB processing pipeline in detail |
| [OCR Pipeline](ocr-pipeline.md) | Tesseract integration, text parsing, and country-specific extractors |
| [Infrastructure](infrastructure.md) | S3, DynamoDB, EventBridge auto-provisioning and IAM permissions |
| [Data Model](data-model.md) | DynamoDB table schemas, field definitions, and data contracts |
| [Event System](event-system.md) | EventBridge event types, payloads, and S3 event store |
| [Configuration](configuration.md) | aws-config.yml, .env, and environment variable reference |
| [Development](development.md) | Local development setup, testing, and CLI dev mode |
| [Deployment](deployment.md) | Building and deploying to AWS Lambda |
| [Security](security.md) | Credential management, IAM policies, and data protection |

---

## 中文

IdentityCardOCR 是一个开源、生产级的 AWS Lambda 服务，用于基于 OCR 的身份证件信息提取。支持中华人民共和国居民身份证和马来西亚 MyKad 证件，具备自动基础设施配置能力。

### 文档索引 / Documentation Index

| 文档 / Document | 描述 / Description |
|----------|-------------|
| [架构设计](architecture.md) | 系统架构、组件关系和设计决策 |
| [Lambda 流程](lambda-flow.md) | S3 事件 → OCR → DynamoDB 处理管道详解 |
| [OCR 管道](ocr-pipeline.md) | Tesseract 集成、文本解析和各国证件提取器 |
| [基础设施](infrastructure.md) | S3、DynamoDB、EventBridge 自动配置和 IAM 权限 |
| [数据模型](data-model.md) | DynamoDB 表结构、字段定义和数据约定 |
| [事件系统](event-system.md) | EventBridge 事件类型、负载和 S3 事件存储 |
| [配置参考](configuration.md) | aws-config.yml、.env 和环境变量参考 |
| [开发指南](development.md) | 本地开发环境搭建、测试和 CLI 开发模式 |
| [部署指南](deployment.md) | 构建和部署到 AWS Lambda |
| [安全架构](security.md) | 凭证管理、IAM 策略和数据保护 |
