# 部署指南

## 构建 AWS Lambda

### 二进制构建

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build -o bootstrap ./cmd/lambda/main.go
```

二进制文件必须命名为 `bootstrap` 以适配 `provided.al2023` Lambda 运行时。必须启用 CGo，因为 gosseract 链接到 Tesseract C 库。

### Lambda 配置

| 设置 | 值 | 备注 |
|---------|-------|-------|
| 运行时 | `provided.al2023` | 自定义运行时（Go 二进制） |
| 架构 | `arm64` (Graviton) | 更低成本，更好性能 |
| 内存 | 最低 512 MB | Tesseract OCR 处理图片需要大量内存 |
| 超时 | 最低 60 秒 | 首次部署时 DynamoDB 表创建需要 15–30 秒 |
| 临时存储 | 512 MB（默认） | 图片下载到 `/tmp` 并在处理后删除 |

### 环境变量

在 Lambda 函数上设置：

```
IS_PRODUCTION=true
LOG_LEVEL=INFO
```

凭证从 Lambda IAM 角色加载——不要在生产环境设置 `AWS_ACCESS_KEY_ID` 或 `AWS_SECRET_ACCESS_KEY`。

### S3 触发器配置

在桶上创建 S3 触发器：
- 事件类型：`s3:ObjectCreated:*`
- 后缀过滤：`.png`、`.jpg`、`.jpeg`

### IAM 角色

附加一个授予 [infrastructure_en.md](infrastructure_en.md) 中记录的权限的策略。

## 部署包

Lambda 部署包需要 Tesseract 共享库：

```bash
# 使用静态链接 Tesseract 构建（推荐，简化部署）
# 或将 .so 文件打包到部署包中
```

对于生产部署，使用基于 Docker 的 Lambda 容器镜像：

```dockerfile
FROM public.ecr.aws/lambda/provided:al2023

RUN dnf install -y tesseract tesseract-langpack-chi-sim tesseract-langpack-eng

COPY bootstrap /var/runtime/
```

## 部署后验证

1. 上传测试图片到 S3 桶：`china/test-id-card.png`
2. 检查 CloudWatch Logs 中的 Lambda 调用日志
3. 在 DynamoDB `identity-card-ocr-users` 表中验证结果
4. 确认 EventBridge 事件已发布到事件总线
