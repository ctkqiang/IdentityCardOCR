# IdentityCardOCR — 文档

开源、生产级 AWS Lambda 服务，用于基于 OCR 的身份证件信息提取。支持中华人民共和国居民身份证和马来西亚 MyKad 证件。

## 目录

| 文档 | 描述 |
|----------|-------------|
| [架构设计](architecture_zh.md) | 系统架构、组件关系和设计决策 |
| [Lambda 流程](lambda-flow_zh.md) | S3 事件到 OCR 到 DynamoDB 处理管道详解 |
| [OCR 管道](ocr-pipeline_zh.md) | Tesseract 集成、文本解析和各国证件提取器 |
| [基础设施](infrastructure_zh.md) | S3、DynamoDB、EventBridge 自动配置和 IAM 权限 |
| [配置参考](configuration_zh.md) | aws-config.yml、.env 和环境变量参考 |
| [开发指南](development_zh.md) | 本地开发环境搭建、测试和 CLI 开发模式 |
| [部署指南](deployment_zh.md) | 构建和部署到 AWS Lambda |

## 快速链接

- 项目标志: [logo.svg](logo.svg)
- English documentation: [index_en.md](index_en.md)
