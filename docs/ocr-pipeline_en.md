# OCR Pipeline

## Overview

The OCR pipeline converts an identity document image into structured fields. It uses Tesseract OCR via the gosseract v2 CGo binding for text extraction, then applies country-specific parsers to extract structured identity information.

## Tesseract Integration

The `ExtractTextFromIdentityDocument` function orchestrates the full OCR-to-structured-data pipeline.

```go
doc, err := service.ExtractTextFromIdentityDocument(
    "/tmp/identity_card.png",
    config.CHINA,
)
```

### Client Configuration

| Parameter | China | Malaysia | US |
|-----------|-------|----------|-----|
| Language | `chi_sim` | `eng` | `eng` |
| PageSegMode | `PSM_SINGLE_BLOCK` | Default | Default |
| Whitelist | `ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789` | Same | Same |

### Text Extraction

The Tesseract client is configured per-country:
1. `SetImage(path)` — loads the image file
2. `SetLanguage(locale)` — sets the OCR language model
3. `SetWhitelist(chars)` — restricts recognition to uppercase letters and digits
4. `SetPageSegMode(mode)` — for Chinese ID cards, `PSM_SINGLE_BLOCK` treats the entire card as one block
5. `Text()` — runs OCR and returns the raw text string

The client is closed via `defer client.Close()` after extraction to release native resources.

## Country-Specific Parsers

### Chinese Resident Identity Card

The `parseChinaIDCard` function extracts:

1. **ID Number (18 digits)** — regex pattern `\b[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`
2. **GB11643-1999 Validation** — full checksum verification with the 17 standard weight factors
3. **Date of Birth** — extracted from ID number positions [6..14)
4. **Sex** — derived from ID number 17th digit parity ("男" / "女")
5. **Region** — administrative region from the 6-digit area code lookup
6. **Name** — longest uppercase alphabetic word preceding the ID number (with noise word filtering)
7. **Expiry Date** — YYYYMMDD date pattern found after the ID number block

### Malaysian MyKad

The `parseMalaysiaMyKad` function extracts:

1. **MyKad Number (12 digits)** — regex pattern `\b\d{12}\b`
2. **Date of Birth** — from MyKad positions [0..6) with century heuristic (YY ≥ 30 → 19YY)
3. **Sex** — from last digit parity ("LELAKI" / "PEREMPUAN")
4. **Name** — uppercase words before the MyKad number (noise-filtered)
5. **Address** — text block between the MyKad number and "WARGANEGARA" marker
6. **Nationality** — hardcoded as "MALAYSIA" (MyKad is exclusive to Malaysian citizens)

## Heuristic Extraction

Name and address fields do not follow a fixed template. The parsers use heuristics tuned for the expected OCR output of each document type:

- **Chinese name**: longest uppercase word (≥2 chars, noise-filtered) before the ID number
- **MyKad name**: sequence of uppercase words (≥3 chars pref., fallback to ≥2) before the MyKad number
- **MyKad address**: all text between the MyKad number and the citizenship marker "WARGANEGARA", with short numeric tokens removed

These heuristics are intentionally simple. For production deployments that require higher accuracy for name/address fields, consider replacing or augmenting these with an LLM-based post-processing step.
