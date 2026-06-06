# 配置参考

## aws-config.yml

AWS 资源名称和区域的主配置文件。位于工作目录或由 `AWS_CONFIG_PATH` 指定路径。

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

### 字段说明

| 字段 | 类型 | 默认值 | 描述 |
|-------|------|---------|-------------|
| `environment.region` | string | `"ap-east-1"` | 所有资源的 AWS 区域 |
| `environment.profile` | string | `"default"` | AWS 凭证配置文件名（仅本地开发） |
| `environment.s3.bucket` | string | `"identity-card-ocr"` | 图片上传和事件存储的 S3 桶名 |
| `environment.s3.path` | string | `"identity/"` | 所有对象的 S3 键前缀 |
| `environment.eventbridge.bus_name` | string | `""`（默认总线） | 自定义事件总线名称；空 = 默认总线 |
| `environment.eventbridge.source` | string | `"identity-card-ocr"` | EventBridge 事件 Source 字段值 |
| `environment.dynamodb.user_identity_table` | string | `"identity-card-ocr-users"` | 成功处理的文档表 |
| `environment.dynamodb.failed_records_table` | string | `"identity-card-ocr-failed"` | 失败 OCR 尝试表 |

### 降级行为

当 `aws-config.yml` 缺失、格式错误或包含空字段时，应用程序降级到上述默认值。应用程序以默认值正常启动运行；不返回错误。

## 环境变量

### .env 文件

```
IS_PRODUCTION=true
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=...
AWS_REGION=ap-east-1
```

| 变量 | 必需 | 默认值 | 描述 |
|----------|----------|---------|-------------|
| `IS_PRODUCTION` | 否 | `false` | 设置为 `"true"` 以在 Lambda 模式运行 |
| `AWS_ACCESS_KEY_ID` | 否 | SDK 链 | AWS 访问密钥（最高优先级凭证源） |
| `AWS_SECRET_ACCESS_KEY` | 否 | SDK 链 | AWS 密钥 |
| `AWS_REGION` | 否 | `ap-east-1` | AWS 区域覆盖 |
| `LOG_LEVEL` | 否 | `INFO` | 日志级别：DEBUG、INFO、WARN、ERROR、VVERBOSE |
| `AWS_CONFIG_PATH` | 否 | `./aws-config.yml` | aws-config.yml 的路径 |
| `DOTENV_PATH` | 否 | `./.env` | .env 文件的路径 |

### 凭证解析顺序

1. `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` 环境变量
2. 工作目录中的 `.env` 文件
3. AWS SDK 默认凭证链：
   - `~/.aws/credentials`
   - `AWS_PROFILE` 环境变量
   - IAM 实例配置文件（EC2）/ IAM 任务角色（ECS/Lambda）
