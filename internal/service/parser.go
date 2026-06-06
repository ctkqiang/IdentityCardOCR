package service

import (
	"identity_card_ocr/internal/config"
	"identity_card_ocr/internal/model"
	"identity_card_ocr/internal/utilities"
	"regexp"
	"strings"
)

// idCardPattern matches an 18-digit Chinese Resident Identity Card number.
// Uses word boundaries to locate the number within OCR text.
var idCardPattern = regexp.MustCompile(`\b[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`)

// myKadPattern matches a 12-digit Malaysian MyKad number.
var myKadPattern = regexp.MustCompile(`\b\d{12}\b`)

// expiryPattern matches dates formatted as YYYYMMDD found near expiry / issue annotations.
var expiryPattern = regexp.MustCompile(`(20\d{2})(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])`)

// ParseOCR extracts structured identity fields from the raw OCR text in doc
// according to the layout conventions of country's identity documents.
//
// For China: parses 18-digit ID number via GB11643-1999, derives DOB/sex/region,
// heuristically extracts the cardholder name and expiry date.
//
// For Malaysia: locates the 12-digit MyKad number, derives DOB via century
// heuristic (YY≥30→19YY), sex via last-digit parity, and heuristically
// extracts name and address blocks.
//
// For unknown countries the input doc is returned unchanged.
func ParseOCR(doc model.DocumentInfo, country config.Country) model.DocumentInfo {
	switch country {
	case config.CHINA:
		return parseChinaIDCard(doc)
	case config.MALAYSIA:
		return parseMalaysiaMyKad(doc)
	default:
		return doc
	}
}

func parseChinaIDCard(doc model.DocumentInfo) model.DocumentInfo {
	raw := doc.RawText

	idNumber := extractIDNumber(raw)
	if idNumber == "" {
		return doc
	}

	info := utilities.ParseIDInfo(idNumber)
	if info == nil {
		return doc
	}

	doc.IDNumber = info.Number
	doc.DateOfBirth = info.DateOfBirth
	doc.Sex = info.Sex
	doc.Nationality = info.Region
	doc.Name = extractChineseName(raw, idNumber)
	doc.ExpiryDate = extractExpiryDate(raw)

	return doc
}

func parseMalaysiaMyKad(doc model.DocumentInfo) model.DocumentInfo {
	raw := doc.RawText

	idNumber := myKadPattern.FindString(raw)
	if idNumber == "" {
		return doc
	}

	doc.IDNumber = idNumber
	doc.DateOfBirth = utilities.MyKadDOB(idNumber)
	doc.Sex = utilities.MyKadSex(idNumber)
	doc.Nationality = "MALAYSIA"
	doc.Name = extractMyKadName(raw, idNumber)
	doc.Address = extractMyKadAddress(raw, idNumber)

	return doc
}

// extractMyKadName extracts the cardholder name from OCR text of a Malaysian MyKad.
// The name appears as uppercase alphabetic words before the 12-digit MyKad number.
func extractMyKadName(raw, idNumber string) string {
	clean := strings.TrimSpace(raw)

	idIdx := strings.Index(clean, idNumber)
	if idIdx <= 0 {
		return ""
	}

	prefix := strings.TrimSpace(clean[:idIdx])

	// Extract uppercase alphabetic words of 3+ characters.
	wordRe := regexp.MustCompile(`\b[A-Z]{3,}\b`)
	candidates := wordRe.FindAllString(prefix, -1)

	// Filter out known OCR artefacts / line markers.
	noiseWords := map[string]bool{
		"JVS": true, "AGS": true, "TCK": true,
	}
	var nameParts []string
	for _, w := range candidates {
		if noiseWords[w] {
			continue
		}
		nameParts = append(nameParts, w)
	}

	if len(nameParts) == 0 {
		// Fallback: try 2-char words, excluding single-letter fragments.
		wordRe2 := regexp.MustCompile(`\b[A-Z]{2,}\b`)
		for _, w := range wordRe2.FindAllString(prefix, -1) {
			if !noiseWords[w] {
				nameParts = append(nameParts, w)
			}
		}
	}

	return strings.Join(nameParts, " ")
}

// extractMyKadAddress extracts the residential address from OCR text of a
// Malaysian MyKad. The address block sits between the MyKad number and the
// citizenship/sex annotations.
func extractMyKadAddress(raw, idNumber string) string {
	clean := strings.TrimSpace(raw)

	idIdx := strings.Index(clean, idNumber)
	if idIdx < 0 {
		return ""
	}

	// Take text after the MyKad number.
	suffix := strings.TrimSpace(clean[idIdx+len(idNumber):])

	// Cut at "WARGANEGARA" (citizenship marker) which ends the address block.
	if idx := strings.Index(suffix, "WARGANEGARA"); idx >= 0 {
		suffix = strings.TrimSpace(suffix[:idx])
	}

	// Collapse whitespace into single spaces and split into logical lines.
	words := strings.Fields(suffix)
	if len(words) == 0 {
		return ""
	}

	// Normalize: strip short noise tokens (1-2 chars, all-digit) from the address.
	var addrParts []string
	for _, w := range words {
		if len(w) <= 2 && isAllDigits(w) {
			continue
		}
		addrParts = append(addrParts, w)
	}

	return strings.Join(addrParts, " ")
}

// isAllDigits returns true if s consists entirely of ASCII digits.
func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// extractIDNumber finds the first 18-digit Chinese ID number in the OCR text.
func extractIDNumber(raw string) string {
	return strings.ToUpper(idCardPattern.FindString(raw))
}

// extractChineseName heuristically extracts a name from the OCR text.
// It looks for the longest alphabetic uppercase word before the ID number.
func extractChineseName(raw, idNumber string) string {
	// If the ID number was found, take the longest uppercase word preceding it.
	clean := strings.TrimSpace(raw)

	// Find ID position
	idIdx := strings.Index(clean, idNumber)
	if idIdx <= 0 {
		return ""
	}

	// Work with text before the ID number
	prefix := strings.TrimSpace(clean[:idIdx])

	// Find uppercase word candidates (at least 2 chars, all caps)
	wordRe := regexp.MustCompile(`\b[A-Z]{2,}\b`)
	candidates := wordRe.FindAllString(prefix, -1)

	// Drop noise like single/double-char fragments (F, M, ES, KE, DB, IT, PS, PSM)
	noiseWords := map[string]bool{
		"ES": true, "DB": true, "IT": true, "KE": true, "PS": true, "PSM": true,
	}
	var longest string
	for _, w := range candidates {
		if noiseWords[w] {
			continue
		}
		if len(w) >= len(longest) {
			longest = w
		}
	}
	return strings.TrimSpace(longest)
}

// extractExpiryDate scans the OCR text for an expiry-related date block.
// On Chinese ID cards the expiry is printed as YYYYMMDD-YYYYMMDD or a single YYYYMMDD.
func extractExpiryDate(raw string) string {
	// Try sex indicator pattern first (e.g. "F19810803") → skip, those are DOB
	// Look for standalone YYYYMMDD dates that appear near "long" numbers
	matches := expiryPattern.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return ""
	}

	// Collect all found YYYYMMDD dates into a slice
	type dateEntry struct {
		full  string
		index int
	}
	var dates []dateEntry
	for _, m := range matches {
		dateStr := m[1] + m[2] + m[3]
		dates = append(dates, dateEntry{full: dateStr, index: strings.Index(raw, dateStr)})
	}

	// The expiry date tends to appear after the ID number and near a second date.
	// If we found >=2 dates, the last pair forms the start-end range.
	if len(dates) >= 2 {
		last := dates[len(dates)-1]
		prev := dates[len(dates)-2]
		return formatDate8(prev.full) + " ~ " + formatDate8(last.full)
	}

	return formatDate8(dates[0].full)
}

// formatDate8 converts YYYYMMDD to YYYY-MM-DD.
func formatDate8(d string) string {
	if len(d) != 8 {
		return d
	}
	return d[0:4] + "-" + d[4:6] + "-" + d[6:8]
}
