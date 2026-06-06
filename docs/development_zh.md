# 开发指南

## 前置条件

- Go 1.26 或更高版本
- Tesseract OCR 5.x
- Tesseract 语言数据：`chi_sim`（简体中文）、`eng`（英语）
- macOS 或 Linux（Tesseract CGo 绑定）

### macOS 安装

```bash
brew install tesseract tesseract-lang
```

### Linux 安装

```bash
sudo apt-get install -y tesseract-ocr tesseract-ocr-chi-sim tesseract-ocr-eng
```

## 本地开发

### 开发模式 CLI

当 `IS_PRODUCTION` 未设置为 `"true"` 时，应用程序启动交互式终端会话：

```bash
go run main.go
```

输出：
```
========================================
  IdentityCardOCR — Dev Mode CLI
========================================
Supported countries: china, malaysia, us
========================================

Enter image path (or 'exit' to quit): sample/china/identity_card.png
Enter country (china/malaysia/us): china

Processing sample/china/identity_card.png (china)...

----------------------------------------
  Extracted Identity Information
----------------------------------------
  ID Number    : 350125199006081234
  Name         : ZHANGSAN
  Nationality  : 福建省福州市永泰县
  Date of Birth: 1990-06-08
  Sex          : 男
  Expiry Date  : 2020-06-08 ~ 2040-06-08
----------------------------------------
  Raw OCR Text :
    姓名 ZHANGSAN 性别 男 民族 汉 ...
----------------------------------------
  [GB11643-1999 Validation: PASSED]
  Region       : 福建省福州市永泰县
  Check Digit  : 4
----------------------------------------
```

### 运行测试

```bash
# 运行 OCR 集成测试
make test

# 运行并输出详细信息
go test -v ./test/ -count=1
```

测试套件处理 `sample/` 中的示例图片并验证：
- 中国身份证：完整 GB11643-1999 校验和、出生日期格式、性别一致性、姓名提取
- 中国护照：原始 OCR 文本提取
- 马来西亚 MyKad：12 位数字模式、出生日期推导、性别推导
- 马来西亚护照：原始 OCR 文本提取

### 构建

```bash
make build
# 输出：bin/identity_card_ocr
```

## 项目布局约定

- `internal/` 包含所有应用逻辑；外部模块不可导入此包之外的任何内容
- 每个包具有单一职责（config、event、pipeline 等）
- `service/aws` 包拥有 AWS 认证单例和基础设施配置功能
- `service/dynamodb` 包是唯一的 DynamoDB 数据访问层
- `utilities` 包无内部依赖——可提取为共享库
