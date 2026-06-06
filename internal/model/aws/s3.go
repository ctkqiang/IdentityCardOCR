package aws

type S3Request struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}
