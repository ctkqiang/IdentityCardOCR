package service

import "github.com/otiai10/gosseract/v2"

func GetTesseractClient() *gosseract.Client {
	return gosseract.NewClient()
}
