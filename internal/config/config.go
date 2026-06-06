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
	} `yaml:"environment"`
}

// AWSConfig holds AWS environment configuration, always read fresh from aws-config.yml.
// Access via config.AWS() to get the latest values at any point in time.
type AWSConfig struct {
	Region  string // AWS region, e.g. ap-east-1 (Hong Kong)
	Profile string // AWS credentials profile name
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
// Use config.AWS().Region / config.AWS().Profile to always get live values.
//
// Falls back to ap-east-1 / default if the file is missing, malformed,
// or fields are empty.
//
// Usage:
//
//	region := config.AWS().Region   // "ap-east-1"
//	profile := config.AWS().Profile // "default"
func AWS() AWSConfig {
	var (
		raw awsConfigRaw
		cfg = AWSConfig{
			Region:  "ap-east-1",
			Profile: "default",
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

	return cfg
}
