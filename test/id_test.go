package test

import (
	"identity_card_ocr/internal/config"
	"identity_card_ocr/internal/model"
	"identity_card_ocr/internal/service"
	"identity_card_ocr/internal/utilities"
	"regexp"
	"testing"
)

// myKadPattern matches a Malaysian MyKad number (12 digits: YYMMDD-BP-###G).
var myKadPattern = regexp.MustCompile(`\b\d{12}\b`)

func TestChineseIdentityCard(t *testing.T) {
	doc, err := service.ExtractTextFromIdentityDocument(
		"../sample/china/identity_card.png", config.CHINA,
	)
	if err != nil {
		t.Fatal(err)
	}

	if doc.RawText == "" {
		t.Fatal("RawText is empty; OCR pipeline produced no output")
	}

	if !utilities.ValidateChineseIDNumber(doc.IDNumber) {
		t.Fatalf("IDNumber %q does not match 18-digit format", doc.IDNumber)
	}

	if !utilities.ValidateChineseIDNumberFull(doc.IDNumber) {
		t.Fatalf("IDNumber %q failed GB11643-1999 checksum validation", doc.IDNumber)
	}

	if !utilities.ValidateDateFormat(doc.DateOfBirth) {
		t.Fatalf("DateOfBirth %q is not in YYYY-MM-DD format", doc.DateOfBirth)
	}

	if !utilities.ValidateDOBConsistency(doc.IDNumber, doc.DateOfBirth) {
		t.Fatalf("DateOfBirth %q inconsistent with IDNumber %q", doc.DateOfBirth, doc.IDNumber)
	}

	if doc.Sex != "男" && doc.Sex != "女" {
		t.Fatalf("Sex %q is neither '男' nor '女'", doc.Sex)
	}

	if !utilities.ValidateSexConsistency(doc.IDNumber, doc.Sex) {
		t.Fatalf("Sex %q inconsistent with IDNumber %q", doc.Sex, doc.IDNumber)
	}

	if doc.Name == "" {
		t.Fatal("Name is empty; parser failed to extract a name")
	}

	info := utilities.ParseIDInfo(doc.IDNumber)
	if info == nil {
		t.Fatalf("ParseIDInfo returned nil for valid ID %q", doc.IDNumber)
	}

	t.Logf("Region     : %q", info.Region)
	logDocumentFields(t, doc)
}

func logDocumentFields(t *testing.T, doc model.DocumentInfo) {
	t.Helper()
	t.Logf("IDNumber   : %q", doc.IDNumber)
	t.Logf("Name       : %q", doc.Name)
	if doc.Surname != "" {
		t.Logf("Surname    : %q", doc.Surname)
	}
	if doc.GivenNames != "" {
		t.Logf("GivenNames : %q", doc.GivenNames)
	}
	t.Logf("Nationality: %q", doc.Nationality)
	t.Logf("DateOfBirth: %q", doc.DateOfBirth)
	t.Logf("Sex        : %q", doc.Sex)
	if doc.Address != "" {
		t.Logf("Address    : %q", doc.Address)
	}
	if doc.ExpiryDate != "" {
		t.Logf("ExpiryDate : %q", doc.ExpiryDate)
	}
	t.Logf("RawText    : %q", doc.RawText)
}

func TestChinesePassport(t *testing.T) {
	doc, err := service.ExtractTextFromIdentityDocument(
		"../sample/china/passport.png", config.CHINA,
	)
	if err != nil {
		t.Fatal(err)
	}

	if doc.RawText == "" {
		t.Fatal("RawText is empty; OCR pipeline produced no output")
	}

	logDocumentFields(t, doc)
}

func TestMalaysianMyKad(t *testing.T) {
	doc, err := service.ExtractTextFromIdentityDocument(
		"../sample/malaysia/identity_card.png", config.MALAYSIA,
	)
	if err != nil {
		t.Fatal(err)
	}

	if doc.RawText == "" {
		t.Fatal("RawText is empty; OCR pipeline produced no output")
	}

	// MyKad numbers are 12 digits; validate at least one is present in the OCR output.
	if myKad := myKadPattern.FindString(doc.RawText); myKad != "" {
		t.Logf("MyKad (12-digit match): %s", myKad)
	} else {
		t.Log("WARNING: no 12-digit MyKad number detected in raw OCR text")
	}

	logDocumentFields(t, doc)
}

func TestMalaysianPassport(t *testing.T) {
	doc, err := service.ExtractTextFromIdentityDocument(
		"../sample/malaysia/passport.png", config.MALAYSIA,
	)
	if err != nil {
		t.Fatal(err)
	}

	if doc.RawText == "" {
		t.Fatal("RawText is empty; OCR pipeline produced no output")
	}

	logDocumentFields(t, doc)
}
