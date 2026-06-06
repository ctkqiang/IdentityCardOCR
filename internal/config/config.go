package config

import (
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
	} `yaml:"environment"`
}

type AWSAuth struct {
	AccessKeyID     AWSSecurity `yaml:"access_key_id"`
	SecretAccessKey string      `yaml:"secret_access_key"`
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

// indexByte returns the index of the first occurrence of b in s, or -1.
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
