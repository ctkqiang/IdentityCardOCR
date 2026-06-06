package aws

import (
	"context"
	"fmt"
	"identity_card_ocr/internal/config"
	"sync"
	"time"

	awshttp "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Account is a stateful, application-wide AWS authentication singleton.
// Initialize once via Init(), then access anywhere via Get().
//
// Thread-safe: all exported methods are guarded by a read-write mutex.
type Account struct {
	cfg      awshttp.Config // reusable AWS SDK config for all service clients
	identity CallerIdentity // verified caller identity from STS
	ready    bool           // true after successful Init()
	mu       sync.RWMutex   // protects all fields
	initErr  error          // captured error from the Init process
}

// CallerIdentity holds the STS GetCallerIdentity response for audit and monitoring.
type CallerIdentity struct {
	AccountID  string    // 12-digit AWS account ID
	ARN        string    // IAM role or user ARN, e.g. arn:aws:sts::123456789012:assumed-role/admin
	UserID     string    // unique user/role ID, e.g. AROA...
	Verified   bool      // true if STS returned a valid identity
	VerifiedAt time.Time // timestamp of last successful STS verification
}

var (
	globalAccount *Account
	globalMu      sync.Mutex
)

// Init performs one-time AWS authentication and stores the result globally.
//
// Configuration is read from:
//   - aws-config.yml  → region, profile
//   - .env / env vars  → AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY (optional)
//
// If no explicit credentials are provided, the AWS SDK falls through
// the default credential chain (~/.aws/credentials, IAM role, etc.).
//
// Call this once during application startup (e.g. in main.go), then
// use Get() everywhere else.
//
// Usage:
//
//	if err := aws.Init(ctx); err != nil {
//	    log.Fatal("AWS authentication failed:", err)
//	}
func Init(ctx context.Context) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalAccount != nil {
		return nil // already initialized
	}

	acct := &Account{}

	// Read access key / secret key from .env (via config.ConfigAWSAuthKeys).
	// If both are present, use static credentials; otherwise fall through
	// to SDK default chain (~/.aws/credentials, IAM role, etc.).
	authKeys := config.ConfigAWSAuthKeys()
	var opts []func(*awsconfig.LoadOptions) error
	if authKeys.AccessKeyID != "" && authKeys.SecretAccessKey != "" {
		opts = append(opts,
			awsconfig.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(
					string(authKeys.AccessKeyID),
					authKeys.SecretAccessKey,
					"", // session token (optional, for STS temporary creds)
				),
			),
		)
	}
	opts = append(opts, awsconfig.WithRegion(config.AWS().Region))

	// Load SDK config with the resolved credentials and region.
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		acct.initErr = fmt.Errorf("aws config load: %w", err)
		globalAccount = acct
		return acct.initErr
	}
	acct.cfg = awsCfg

	// Verify credentials by calling STS GetCallerIdentity.
	// This confirms the credentials are valid and retrieves
	// the caller's account ID, ARN, and user ID for auditing.
	stsClient := sts.NewFromConfig(awsCfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		acct.initErr = fmt.Errorf("sts get-caller-identity: %w", err)
		globalAccount = acct
		return acct.initErr
	}

	acct.identity = CallerIdentity{
		AccountID:  awshttp.ToString(identity.Account),
		ARN:        awshttp.ToString(identity.Arn),
		UserID:     awshttp.ToString(identity.UserId),
		Verified:   true,
		VerifiedAt: time.Now(),
	}
	acct.ready = true

	globalAccount = acct
	return nil
}

// Get returns the global authenticated Account singleton.
//
// Panics if Init() has not been called successfully.
// Call Init() once during startup before any Get() calls.
//
// Usage:
//
//	acct := aws.Get()
//	s3Client := s3.NewFromConfig(acct.Config())
func GetAccount() *Account {
	globalMu.Lock()
	acct := globalAccount
	globalMu.Unlock()

	if acct == nil {
		panic("aws: Init() must be called before Get()")
	}
	return acct
}

// Ready returns true if authentication succeeded and the Account is usable.
// Safe to call before Get() to check state without panicking.
func Ready() bool {
	globalMu.Lock()
	defer globalMu.Unlock()
	return globalAccount != nil && globalAccount.ready
}

// InitError returns the error from the last Init() call, or nil if successful.
func InitError() error {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalAccount == nil {
		return nil
	}
	return globalAccount.initErr
}

// Config returns the reusable AWS SDK config for constructing service clients.
//
// All AWS service clients throughout the application share this single config,
// ensuring consistent region, credentials, and HTTP settings.
//
// Usage:
//
//	s3Client  := s3.NewFromConfig(aws.Get().Config())
//	txtClient := textract.NewFromConfig(aws.Get().Config())
func (a *Account) Config() awshttp.Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg
}

// Identity returns the verified STS caller identity.
// Use for logging, auditing, and confirming which AWS account/role is active.
//
// Usage:
//
//	id := aws.Get().Identity()
//	log.Printf("Running as %s (account %s)", id.ARN, id.AccountID)
func (a *Account) Identity() CallerIdentity {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.identity
}

// Reauth re-runs STS GetCallerIdentity to refresh the caller identity.
// Useful after credential rotation or long-running processes.
//
// Returns the updated identity on success.
func (a *Account) Reauth(ctx context.Context) (CallerIdentity, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	stsClient := sts.NewFromConfig(a.cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return a.identity, fmt.Errorf("sts reauth: %w", err)
	}

	a.identity = CallerIdentity{
		AccountID:  awshttp.ToString(identity.Account),
		ARN:        awshttp.ToString(identity.Arn),
		UserID:     awshttp.ToString(identity.UserId),
		Verified:   true,
		VerifiedAt: time.Now(),
	}

	return a.identity, nil
}

// AWSSdkClient returns the shared AWS SDK config from the authenticated Account singleton.
//
// Call Init(ctx) once in main() before using this function.
// Returns the cached config — no redundant LoadDefaultConfig calls.
//
// Usage:
//
//	cfg, err := aws.AWSSdkClient(ctx)
//	s3Client := s3.NewFromConfig(cfg)
func AWSSdkClient(_ context.Context) (awshttp.Config, error) {
	if !Ready() {
		return awshttp.Config{}, fmt.Errorf("aws: not authenticated — call Init() first")
	}
	return GetAccount().Config(), nil
}

// Dispose clears the global singleton.
// Call only during graceful shutdown or testing teardown.
func Dispose() {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalAccount = nil
}
