package model

type DocumentInfo struct {
	IDNumber    string `json:"id_number"`
	Name        string `json:"name"`
	Surname     string `json:"surname,omitempty"`
	GivenNames  string `json:"given_names,omitempty"`
	Nationality string `json:"nationality,omitempty"`
	DateOfBirth string `json:"date_of_birth"`
	Sex         string `json:"sex"`
	Address     string `json:"address,omitempty"`
	ExpiryDate  string `json:"expiry_date,omitempty"`
	RawText     string `json:"raw_text,omitempty"`
}

type ChineseIdentityCardInfo struct {
	Document  DocumentInfo `json:"document"`
	Province  *string      `json:"province"`
	City      *string      `json:"city"`
	District  *string      `json:"district"`
	Community *string      `json:"community"`
	Street    *string      `json:"street"`
}
