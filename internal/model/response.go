package model

type Response struct {
	Text     string       `json:"text"`
	Document DocumentInfo `json:"document_info,omitempty"`
	Err      *error       `json:"err,omitempty"`
}
