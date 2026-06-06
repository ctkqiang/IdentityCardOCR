package model

type Response struct {
	Text string `json:"text"`
	Err  string `json:"err,omitempty"`
}
