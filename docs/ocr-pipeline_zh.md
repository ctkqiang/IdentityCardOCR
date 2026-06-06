# OCR 管道

## 概述

OCR 管道将身份证件图片转换为结构化字段。它使用 Tesseract OCR（通过 gosseract v2 CGo 绑定）进行文本提取，然后应用各国特定的解析器提取结构化身份信息。

## Tesseract 集成

`ExtractTextFromIdentityDocument` 函数编排从 OCR 到结构化数据的完整管道。

```go
doc, err := service.ExtractTextFromIdentityDocument(
    "/tmp/identity_card.png",
    config.CHINA,
)
```

### 客户端配置

| 参数 | 中国 | 马来西亚 | 美国 |
|-----------|-------|----------|-----|
| 语言 | `chi_sim` | `eng` | `eng` |
| PageSegMode | `PSM_SINGLE_BLOCK` | 默认 | 默认 |
| 白名单 | `ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789` | 同上 | 同上 |

### 文本提取

Tesseract 客户端按国家配置：
1. `SetImage(path)` — 加载图像文件
2. `SetLanguage(locale)` — 设置 OCR 语言模型
3. `SetWhitelist(chars)` — 限制识别为大写字母和数字
4. `SetPageSegMode(mode)` — 中国身份证使用 `PSM_SINGLE_BLOCK` 将整张卡片视为一个块
5. `Text()` — 运行 OCR 并返回原始文本字符串

客户端在提取后通过 `defer client.Close()` 关闭以释放原生资源。

## 各国特定解析器

### 中国居民身份证

`parseChinaIDCard` 函数提取：

1. **身份证号（18 位）** — 正则模式 `\b[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`
2. **GB11643-1999 验证** — 使用 17 个标准加权因子进行完整校验和验证
3. **出生日期** — 从身份证号位置 [6..14) 提取
4. **性别** — 从身份证号第 17 位奇偶推导（"男" / "女"）
5. **地区** — 从 6 位地区代码查找行政区划
6. **姓名** — 身份证号前最长的连续大写字母单词（已过滤噪声词）
7. **有效期** — 身份证号块后找到的 YYYYMMDD 日期模式

### 马来西亚 MyKad

`parseMalaysiaMyKad` 函数提取：

1. **MyKad 号码（12 位）** — 正则模式 `\b\d{12}\b`
2. **出生日期** — 从 MyKad 位置 [0..6) 提取，世纪推断（YY ≥ 30 → 19YY）
3. **性别** — 从末位奇偶判断（"LELAKI" / "PEREMPUAN"）
4. **姓名** — MyKad 号前的大写单词（已过滤噪声）
5. **地址** — MyKad 号与 "WARGANEGARA" 标记之间的文本块
6. **国籍** — 硬编码为 "MALAYSIA"（MyKad 仅限马来西亚公民）

## 启发式提取

姓名和地址字段不遵循固定模板。解析器使用针对每种证件类型的预期 OCR 输出调优的启发式方法：

- **中文姓名**：身份证号前最长的连续大写单词（≥2 字符，已过滤噪声）
- **MyKad 姓名**：MyKad 号前的大写单词序列（优先 ≥3 字符，降级到 ≥2 字符）
- **MyKad 地址**：MyKad 号与国籍标记 "WARGANEGARA" 之间的所有文本，去除短数字标记

这些启发式方法故意保持简单。对于需要更高姓名/地址字段准确率的生产部署，考虑用基于 LLM 的后处理步骤替换或增强这些方法。
