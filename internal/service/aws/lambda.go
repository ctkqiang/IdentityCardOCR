package aws

// Lambda handler is defined in internal/lambda/handler.go (HandleRequest).
// The root main.go and cmd/lambda/main.go wire it directly.
//
// This file is intentionally empty — the aws package provides
// authentication (account.go), infrastructure (infra.go), and S3 (s3.go)
// but does not own the Lambda handler itself.
