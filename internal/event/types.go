package event

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"
)

// EventType identifies the kind of domain event.
type EventType string

const (
	DocumentSubmitted   EventType = "document.submitted"
	ProcessingCompleted EventType = "processing.completed"
	ProcessingFailed    EventType = "processing.failed"
)

const eventSource = "identity-card-ocr"

// Event is the envelope for all domain events in the system.
// Detail carries the type-specific payload as raw JSON.
type Event struct {
	ID         string          `json:"id"`
	Type       EventType       `json:"type"`
	Source     string          `json:"source"`
	Timestamp  time.Time       `json:"timestamp"`
	DocumentID string          `json:"document_id"`
	Detail     json.RawMessage `json:"detail"`
}

// DocumentSubmittedPayload is the detail for document.submitted events.
type DocumentSubmittedPayload struct {
	ImagePath string `json:"image_path,omitempty"`
	S3Bucket  string `json:"s3_bucket,omitempty"`
	S3Key     string `json:"s3_key,omitempty"`
	Country   string `json:"country"`
}

// ProcessingCompletedPayload is the detail for processing.completed events.
type ProcessingCompletedPayload struct {
	IDNumber    string `json:"id_number"`
	Name        string `json:"name"`
	Nationality string `json:"nationality,omitempty"`
	DateOfBirth string `json:"date_of_birth"`
	Sex         string `json:"sex"`
	ExpiryDate  string `json:"expiry_date,omitempty"`
	RawText     string `json:"raw_text,omitempty"`
}

// ProcessingFailedPayload is the detail for processing.failed events.
type ProcessingFailedPayload struct {
	Error string `json:"error"`
	Phase string `json:"phase"`
}

// New creates an Event with an auto-generated UUID, current UTC timestamp,
// and the detail value marshaled to JSON.
func New(typ EventType, documentID string, detail interface{}) Event {
	raw, err := json.Marshal(detail)
	if err != nil {
		raw = json.RawMessage(fmt.Sprintf(`{"error":"marshal failed: %s"}`, err.Error()))
	}
	return Event{
		ID:         NewID(),
		Type:       typ,
		Source:     eventSource,
		Timestamp:  time.Now().UTC(),
		DocumentID: documentID,
		Detail:     raw,
	}
}

// UnmarshalDetail decodes the Detail field into v.
func (e *Event) UnmarshalDetail(v interface{}) error {
	return json.Unmarshal(e.Detail, v)
}

// NewID generates a UUID v4 string using crypto/rand.
func NewID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	buf[6] = (buf[6] & 0x0f) | 0x40 // version 4
	buf[8] = (buf[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}
