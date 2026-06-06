package config

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

type Country string

const (
	CHINA    Country = "CN"
	MALAYSIA Country = "MY"
	US       Country = "US"
)

// awsConfigRaw mirrors the structure of aws-config.yml for YAML unmarshaling.
type awsConfigRaw struct {
	Environment struct {
		Region  string `yaml:"region"`
		Profile string `yaml:"profile"`
		S3      struct {
			Bucket string `yaml:"bucket"`
			Path   string `yaml:"path"`
		} `yaml:"s3"`
	} `yaml:"environment"`
}

// S3Config holds S3 bucket and key prefix from aws-config.yml.
type S3Config struct {
	Bucket string // S3 bucket name, e.g. identity-card-ocr
	Path   string // S3 key prefix, e.g. identity/
}

// AWSConfig holds AWS environment configuration, always read fresh from aws-config.yml.
// Access via config.AWS().Region / config.AWS().S3.Bucket to always get live values.
type AWSConfig struct {
	Region  string   // AWS region, e.g. ap-east-1 (Hong Kong)
	Profile string   // AWS credentials profile name
	S3      S3Config // S3 bucket and path configuration
}

var (
	awsConfigPath     string
	awsConfigPathOnce sync.Once
)

// resolveAWSConfigPath determines the aws-config.yml path once.
// Priority: AWS_CONFIG_PATH env → $CWD/aws-config.yml → aws-config.yml
func resolveAWSConfigPath() {
	awsConfigPathOnce.Do(func() {
		awsConfigPath = os.Getenv("AWS_CONFIG_PATH")
		if awsConfigPath != "" {
			return
		}

		cwd, err := os.Getwd()
		if err == nil {
			awsConfigPath = filepath.Join(cwd, "aws-config.yml")
		} else {
			awsConfigPath = "aws-config.yml"
		}
	})
}

// AWS reads aws-config.yml and returns the current AWS configuration.
// Use config.AWS().Region / config.AWS().S3.Bucket to always get live values.
//
// Falls back to safe defaults if the file is missing, malformed,
// or fields are empty.
//
// Usage:
//
//	region  := config.AWS().Region       // "ap-east-1"
//	bucket  := config.AWS().S3.Bucket    // "identity-card-ocr"
//	keyPath := config.AWS().S3.Path      // "identity/"
func AWS() AWSConfig {
	var (
		raw awsConfigRaw
		cfg = AWSConfig{
			Region:  "ap-east-1",
			Profile: "default",
			S3: S3Config{
				Bucket: "identity-card-ocr",
				Path:   "identity/",
			},
		}
	)

	resolveAWSConfigPath()

	data, err := os.ReadFile(awsConfigPath)
	if err != nil {
		return cfg
	}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return cfg
	}

	if raw.Environment.Region != "" {
		cfg.Region = raw.Environment.Region
	}

	if raw.Environment.Profile != "" {
		cfg.Profile = raw.Environment.Profile
	}

	if raw.Environment.S3.Bucket != "" {
		cfg.S3.Bucket = raw.Environment.S3.Bucket
	}

	if raw.Environment.S3.Path != "" {
		cfg.S3.Path = raw.Environment.S3.Path
	}

	return cfg
}
