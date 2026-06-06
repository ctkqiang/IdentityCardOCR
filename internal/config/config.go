package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type (
	Country      int
	AWSSecurity  string
	Locale       string
	DocumentType int
)

func (l Locale) String() string {
	panic("unimplemented")
}

const (
	CHINA Country = iota
	MALAYSIA
	US
)

func (c Country) String() string {
	switch c {
	case CHINA:
		return "china"
	case MALAYSIA:
		return "malaysia"
	case US:
		return "us"
	default:
		return "unknown"
	}
}

const (
	CHINESE Locale = "chi_sim"
	ENGLISH Locale = "eng"
)

const (
	ChinaIDCard DocumentType = iota
	MalaysianMYKAD
	ChinesePassport
	MalaysianPassport
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
		EventBridge struct {
			BusName string `yaml:"bus_name"`
			Source  string `yaml:"source"`
		} `yaml:"eventbridge"`
		DynamoDB struct {
			UserIdentityTable string `yaml:"user_identity_table"`
			FailedRecordsTable string `yaml:"failed_records_table"`
		} `yaml:"dynamodb"`
	} `yaml:"environment"`
}

type AWSAuth struct {
	AccessKeyID     AWSSecurity `yaml:"access_key_id"`
	SecretAccessKey string      `yaml:"secret_access_key"`
}

// S3Config holds the S3 bucket name and key prefix used for image uploads
// and the append-only event log. Both values originate from aws-config.yml
// and are read fresh on each config.AWS() call.
type S3Config struct {
	Bucket string // S3 bucket name, e.g. identity-card-ocr
	Path   string // S3 key prefix without trailing slash, e.g. identity
}

// EventBridgeConfig holds the custom event bus name and the Source field
// value attached to every published event. When BusName is empty, events
// are published to the default event bus.
type EventBridgeConfig struct {
	BusName string // custom event bus name; empty string selects the default bus
	Source  string // value written to the Source field of every EventBridge event
}

// DynamoDBConfig holds the table names used for persisting OCR results.
// UserIdentityTable receives successfully parsed identity documents.
// FailedRecordsTable receives failed OCR attempts with phase and error details.
type DynamoDBConfig struct {
	UserIdentityTable  string // table for passed OCR results (PK: document_id)
	FailedRecordsTable string // table for failed OCR attempts (PK: document_id)
}

// AWSConfig is the root AWS environment configuration, deserialized from
// aws-config.yml on every call to config.AWS(). Callers receive the current
// live values; there is no in-memory cache that can drift from the file.
//
// Field access pattern:
//
//	region  := config.AWS().Region
//	bucket  := config.AWS().S3.Bucket
//	keyPath := config.AWS().S3.Path
//
// All fields fall back to safe defaults when aws-config.yml is missing,
// malformed, or contains empty values.
type AWSConfig struct {
	Region      string            // AWS region, e.g. ap-east-1 (Hong Kong)
	Profile     string            // AWS credentials profile name for local development
	S3          S3Config          // S3 bucket and key-prefix configuration
	EventBridge EventBridgeConfig // EventBridge bus name and event source
	DynamoDB    DynamoDBConfig    // DynamoDB table names for OCR results
}

var (
	awsConfigPath     string
	awsConfigPathOnce sync.Once
)

// resolveAWSConfigPath resolves the absolute path to aws-config.yml exactly once
// per process lifetime. Resolution order: AWS_CONFIG_PATH environment variable,
// then $CWD/aws-config.yml, then ./aws-config.yml relative to the binary.
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

// AWS reads aws-config.yml from disk and returns the merged AWS configuration.
//
// Every call re-reads the file so configuration changes take effect on the
// next invocation without a restart. Missing or malformed YAML causes a
// silent fallback to the documented defaults.
//
// The caller must not modify the returned struct; it is a value copy.
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
			EventBridge: EventBridgeConfig{
				BusName: "",
				Source:  "identity-card-ocr",
			},
			DynamoDB: DynamoDBConfig{
				UserIdentityTable: "identity-card-ocr-users",
				FailedRecordsTable: "identity-card-ocr-failed",
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

	if raw.Environment.EventBridge.BusName != "" {
		cfg.EventBridge.BusName = raw.Environment.EventBridge.BusName
	}

	if raw.Environment.EventBridge.Source != "" {
		cfg.EventBridge.Source = raw.Environment.EventBridge.Source
	}

	if raw.Environment.DynamoDB.UserIdentityTable != "" {
		cfg.DynamoDB.UserIdentityTable = raw.Environment.DynamoDB.UserIdentityTable
	}

	if raw.Environment.DynamoDB.FailedRecordsTable != "" {
		cfg.DynamoDB.FailedRecordsTable = raw.Environment.DynamoDB.FailedRecordsTable
	}

	return cfg
}

// ConfigAWSAuthKeys reads AWS credentials from environment variables and .env,
// returning an AWSAuth struct suitable for SDK credential configuration.
//
// Resolution order:
//   - os.Getenv("AWS_ACCESS_KEY_ID") / os.Getenv("AWS_SECRET_ACCESS_KEY")
//   - .env file in the working directory
//   - zero-value AWSAuth (SDK falls through to the default credential chain)
//
// The returned AccessKeyID and SecretAccessKey may both be empty, signalling
// the caller to rely on the AWS SDK default credential provider chain
// (~/.aws/credentials, IAM instance profile, ECS task role, etc.).
func ConfigAWSAuthKeys() AWSAuth {
	return loadEnvAuth()
}

// loadEnvAuth reads AWS credentials from .env file in the working directory.
// Priority: os.Getenv → .env file → empty string (AWS SDK will use default credential chain).
//
// .env format expected:
//
//	AWS_ACCESS_KEY_ID=AKIA...
//	AWS_SECRET_ACCESS_KEY=...
//	AWS_REGION=ap-east-1
//
// Lines starting with # are treated as comments and ignored.
// Blank lines and malformed lines (missing =) are skipped silently.
func loadEnvAuth() AWSAuth {
	auth := AWSAuth{
		AccessKeyID:     AWSSecurity(os.Getenv("AWS_ACCESS_KEY_ID")),
		SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}

	// If env vars are already set, use them directly (highest priority).
	if auth.AccessKeyID != "" && auth.SecretAccessKey != "" {
		return auth
	}

	// Fallback: parse .env file from working directory.
	envPath := resolveDotEnvPath()
	data, err := os.ReadFile(envPath)
	if err != nil {
		return auth
	}

	for _, line := range splitLines(string(data)) {
		line = trimInlineComment(line)
		if line == "" {
			continue
		}
		eq := indexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := line[:eq]
		val := line[eq+1:]

		switch key {
		case "AWS_ACCESS_KEY_ID":
			if auth.AccessKeyID == "" {
				auth.AccessKeyID = AWSSecurity(val)
			}
		case "AWS_SECRET_ACCESS_KEY":
			if auth.SecretAccessKey == "" {
				auth.SecretAccessKey = val
			}
		}
	}

	return auth
}

var (
	dotEnvPath     string
	dotEnvPathOnce sync.Once
)

func resolveDotEnvPath() string {
	dotEnvPathOnce.Do(func() {
		dotEnvPath = os.Getenv("DOTENV_PATH")
		if dotEnvPath != "" {
			return
		}
		cwd, err := os.Getwd()
		if err == nil {
			dotEnvPath = filepath.Join(cwd, ".env")
		} else {
			dotEnvPath = ".env"
		}
	})
	return dotEnvPath
}

// splitLines splits a string by \n, preserving empty lines for accurate parsing.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// trimInlineComment removes # comments from a .env line.
// Only trims if # is preceded by whitespace or at start of line.
func trimInlineComment(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
			return strings.TrimRight(s[:i], " \t")
		}
	}
	return s
}

// CountryFromString converts a lowercase country identifier to its Country
// enum constant. Accepted inputs are "china", "malaysia", and "us".
//
// The returned error reports the unrecognised input string when the country
// is not one of the three supported values.
func CountryFromString(s string) (Country, error) {
	switch s {
	case "china":
		return CHINA, nil
	case "malaysia":
		return MALAYSIA, nil
	case "us":
		return US, nil
	default:
		return -1, fmt.Errorf("unknown country: %s", s)
	}
}

// indexByte returns the index of the first occurrence of b in s, or -1.
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
