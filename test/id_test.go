package test

import (
	"identity_card_ocr/internal/config"
	"identity_card_ocr/internal/service"
	"identity_card_ocr/internal/utilities"
	"testing"
)

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
		t.Fatalf("DateOfBirth %q is inconsistent with IDNumber %q", doc.DateOfBirth, doc.IDNumber)
	}

	if doc.Sex != "男" && doc.Sex != "女" {
		t.Fatalf("Sex %q is neither '男' nor '女'", doc.Sex)
	}

	if !utilities.ValidateSexConsistency(doc.IDNumber, doc.Sex) {
		t.Fatalf("Sex %q is inconsistent with IDNumber %q", doc.Sex, doc.IDNumber)
	}

	if doc.Name == "" {
		t.Fatal("Name is empty; parser failed to extract a name")
	}

	info := utilities.ParseIDInfo(doc.IDNumber)
	if info == nil {
		t.Fatalf("ParseIDInfo returned nil for valid ID %q", doc.IDNumber)
	}

	t.Logf("ID=%s DOB=%s Sex=%s Name=%s Expiry=%s Region=%s",
		doc.IDNumber,
		doc.DateOfBirth,
		doc.Sex,
		doc.Name,
		doc.ExpiryDate,
		info.Region,
	)
}
