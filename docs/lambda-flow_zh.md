# Lambda 处理流程

## 入口点

Lambda 函数由 S3 `ObjectCreated:*` 事件触发。每次调用接收一个 `events.S3Event`，包含一个或多个 S3 对象记录。

## 处理器生命周期

### 冷启动 (main.go)

```
aws.Init()                   → STS 认证，缓存 SDK 配置
aws.EnsureInfrastructure()   → 验证/创建 S3 桶、DynamoDB 表、EventBridge 总线
lambda.Start(HandleRequest)  → 进入 Lambda 运行时循环（永久阻塞）
```

### 每次调用 (HandleRequest)

对事件中的每个 S3 对象：

```
1. inferCountry(key)
   └─ 解析 "china/..." 或 "malaysia/..." 前缀
   └─ 失败 → emitFailure("init")，继续下一个对象

2. downloadS3Object(bucket, key, /tmp/{filename})
   └─ 失败 → emitFailure("init")，继续下一个对象

3. service.ExtractTextFromIdentityDocument(/tmp/{filename}, country)
   ├─ GetTesseractClient()
   ├─ SetImage(path)
   ├─ SetLanguage(chi_sim / eng)
   ├─ Text() → 原始 OCR 字符串
   ├─ ParseOCR(rawText, country)
   │   ├─ [中国]  parseChinaIDCard()
   │   │   ├─ extractIDNumber()      → 18 位正则
   │   │   ├─ ParseIDInfo()          → GB11643-1999 验证
   │   │   ├─ extractChineseName()   → 启发式姓名提取
   │   │   └─ extractExpiryDate()    → 日期范围提取
   │   └─ [马来西亚] parseMalaysiaMyKad()
   │       ├─ MyKadDOB()             → YYMMDD → YYYY-MM-DD
   │       ├─ MyKadSex()             → 末位奇偶判断性别
   │       ├─ extractMyKadName()     → 大写单词启发式提取
   │       └─ extractMyKadAddress()  → ID 后地址块
   └─ 返回 DocumentInfo

4. [成功]
   ├─ pipe.Process(processing.completed 事件)
   │   ├─ store.Append()      → S3 {prefix}/events/{docID}/{ts}.json
   │   └─ bus.Put()           → EventBridge
   └─ ddb.PutUserIdentity()   → DynamoDB user_identity 表

5. [失败]
   ├─ pipe.Process(processing.failed 事件)
   │   ├─ store.Append()      → S3 事件日志
   │   └─ bus.Put()           → EventBridge
   └─ ddb.PutFailedRecord()   → DynamoDB failed_records 表

6. os.Remove(/tmp/{filename})

7. 继续处理下一个 S3 对象
```

## 响应格式

```json
{
  "message": "processed 3 objects",
  "total": 3,
  "passed": 2,
  "failed": 1
}
```

## 国家推断

文档国家从 S3 对象键前缀推断：

| 键前缀 | 国家 |
|-----------|---------|
| `china/...` | 中国 (chi_sim OCR, GB11643-1999 验证) |
| `malaysia/...` | 马来西亚 (eng OCR, MyKad 规则) |
| `us/...` | 美国 (eng OCR, 无特定解析器) |
| 未知前缀 | 失败 (`"init"` 阶段) |

## 并发

每个 S3 事件触发一次 Lambda 调用。同一事件中的多个 S3 对象在调用内顺序处理。并发上传触发独立的 Lambda 调用，并行运行。

## 临时文件

图片下载到 `/tmp`（Lambda 临时存储）。OCR 处理后立即删除文件，避免超过 512 MB 的存储限制。
