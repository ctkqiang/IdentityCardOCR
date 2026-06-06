package aws

import (
	"context"
	"fmt"
	"identity_card_ocr/internal/config"
	"identity_card_ocr/internal/utilities"
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

// Init authenticates with AWS and caches the resulting SDK configuration
// in a process-wide singleton. It is safe to call multiple times; subsequent
// calls after the first successful authentication return nil immediately.
//
// Credential resolution merges three sources:
//   - config.ConfigAWSAuthKeys() for static credentials from .env
//   - config.AWS().Region for the target region from aws-config.yml
//   - AWS SDK default credential chain (~/.aws/credentials, IAM role, etc.)
//
// The caller receives an STS GetCallerIdentity-verified error or nil.
// On success, the authenticated account identity (ARN, user ID, account ID)
// is logged and available via GetAccount().Identity().
func Init(ctx context.Context) error {
	var opts []func(*awsconfig.LoadOptions) error

	globalMu.Lock()
	defer globalMu.Unlock()

	if globalAccount != nil {
		return nil
	}

	acct := &Account{}

	// Read access key / secret key from .env (via config.ConfigAWSAuthKeys).
	// If both are present, use static credentials; otherwise fall through
	// to SDK default chain (~/.aws/credentials, IAM role, etc.).
	authKeys := config.ConfigAWSAuthKeys()
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

	utilities.LogProgress("aws", "init", "success",
		fmt.Sprintf("account %s, user %s, arn %s",
			acct.identity.AccountID,
			acct.identity.UserID,
			acct.identity.ARN,
		),
	)
	acct.ready = true
	globalAccount = acct

	return nil
}

// GetAccount returns the process-wide authenticated Account singleton.
//
// The caller must have called Init() successfully before invoking GetAccount.
// If Init() has not been called or failed, GetAccount panics.
// Use Ready() to check authentication state without panicking.
//
// The returned pointer is valid for the lifetime of the process; the
// underlying SDK configuration is immutable after Init() completes.
func GetAccount() *Account {
	globalMu.Lock()
	acct := globalAccount
	globalMu.Unlock()

	if acct == nil {
		panic("aws: Init() must be called before Get()")
	}
	return acct
}

// Ready reports whether Init() has completed successfully and the Account
// singleton is safe to use. Callers that must avoid panics should guard
// GetAccount() with Ready().
func Ready() bool {
	globalMu.Lock()
	defer globalMu.Unlock()
	return globalAccount != nil && globalAccount.ready
}

// InitError returns the error captured during the most recent Init() call,
// or nil when Init() succeeded or has not been invoked yet.
func InitError() error {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalAccount == nil {
		return nil
	}
	return globalAccount.initErr
}

// Config returns the shared AWS SDK configuration used by all service clients
// in the application. The returned config carries the authenticated region,
// credentials, and HTTP client settings established during Init().
//
// The config is immutable after Init(); callers receive a value copy.
func (a *Account) Config() awshttp.Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg
}

// Identity returns the STS GetCallerIdentity response captured during Init().
// The returned struct contains the verified AWS account ID, IAM ARN, and
// user ID of the caller. It is read-only; call Reauth() to refresh the
// identity mid-process.
func (a *Account) Identity() CallerIdentity {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.identity
}

// Reauth re-runs STS GetCallerIdentity against the existing SDK configuration
// and replaces the cached identity. Useful after credential rotation or in
// long-running processes that need periodic identity verification.
//
// The call is safe for concurrent use; it acquires the write lock.
// On error the previous identity is preserved.
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

// AWSSdkClient returns the shared SDK configuration from the authenticated
// Account singleton. Init() must be called before invoking this function.
//
// The returned config is the single cached value from Init(); no additional
// HTTP calls or credential resolution occurs.
func AWSSdkClient(_ context.Context) (awshttp.Config, error) {
	if !Ready() {
		return awshttp.Config{}, fmt.Errorf("aws: not authenticated — call Init() first")
	}
	return GetAccount().Config(), nil
}

// Dispose clears the global Account singleton, releasing the SDK
// configuration. Intended for graceful shutdown and test teardown only.
// After Dispose(), Ready() reports false until Init() is called again.
func Dispose() {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalAccount = nil
}
