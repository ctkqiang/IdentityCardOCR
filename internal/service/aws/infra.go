package aws

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"identity_card_ocr/internal/config"
	"identity_card_ocr/internal/utilities"
)

// EnsureS3Bucket verifies the named S3 bucket exists in the given region.
// If the bucket does not exist, it is created with default private ACL.
// Returns nil if the bucket already exists or was successfully created.
func EnsureS3Bucket(ctx context.Context, cfg aws.Config, bucket, region string) error {
	s3c := s3.NewFromConfig(cfg)

	_, err := s3c.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err == nil {
		utilities.LogProgress("infra", "s3", "bucket exists", "bucket="+bucket)
		return nil
	}

	utilities.LogProgress("infra", "s3", "creating bucket", "bucket="+bucket, "region="+region)

	createInput := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}
	if region != "us-east-1" {
		createInput.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(region),
		}
	}

	if _, err := s3c.CreateBucket(ctx, createInput); err != nil {
		if isBucketAlreadyOwnedByYou(err) {
			utilities.LogProgress("infra", "s3", "bucket already owned", "bucket="+bucket)
			return nil
		}
		return fmt.Errorf("create s3 bucket %q: %w", bucket, err)
	}

	if err := s3.NewBucketExistsWaiter(s3c).Wait(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	}, 30*time.Second); err != nil {
		utilities.LogProgress("infra", "s3", "bucket created but wait timed out", "bucket="+bucket)
	}

	utilities.LogProgress("infra", "s3", "bucket created", "bucket="+bucket, "region="+region)
	return nil
}

// apiError matches smithy.APIError for SDK error code extraction.
type apiError interface {
	ErrorCode() string
}

// isBucketAlreadyOwnedByYou checks if the S3 error indicates the bucket
// already exists and is owned by this AWS account.
func isBucketAlreadyOwnedByYou(err error) bool {
	var ae apiError
	if errors.As(err, &ae) {
		code := ae.ErrorCode()
		return code == "BucketAlreadyOwnedByYou" || code == "BucketAlreadyExists"
	}
	return false
}

// EnsureEventBridgeBus verifies the named custom event bus exists.
// If busName is empty (default bus), this is a no-op.
// Creates the bus if it doesn't exist.
func EnsureEventBridgeBus(ctx context.Context, cfg aws.Config, busName string) error {
	if busName == "" {
		utilities.LogProgress("infra", "eventbridge", "using default event bus")
		return nil
	}

	ebc := eventbridge.NewFromConfig(cfg)

	_, err := ebc.DescribeEventBus(ctx, &eventbridge.DescribeEventBusInput{
		Name: aws.String(busName),
	})
	if err == nil {
		utilities.LogProgress("infra", "eventbridge", "bus exists", "bus="+busName)
		return nil
	}

	utilities.LogProgress("infra", "eventbridge", "creating bus", "bus="+busName)
	if _, err := ebc.CreateEventBus(ctx, &eventbridge.CreateEventBusInput{
		Name: aws.String(busName),
	}); err != nil {
		return fmt.Errorf("create event bus %q: %w", busName, err)
	}

	utilities.LogProgress("infra", "eventbridge", "bus created", "bus="+busName)
	return nil
}

// ddbTableSchema defines the key schema and attribute definitions for a DynamoDB table.
type ddbTableSchema struct {
	TableName  string
	HashKey    string
	RangeKey   string // empty if no sort key
	Attributes []ddbtypes.AttributeDefinition
	KeySchema  []ddbtypes.KeySchemaElement
}

// newSimpleTable returns a schema for a table with a single string hash key.
func newSimpleTable(name, hashKey string) ddbTableSchema {
	return ddbTableSchema{
		TableName: name,
		HashKey:   hashKey,
		Attributes: []ddbtypes.AttributeDefinition{
			{
				AttributeName: aws.String(hashKey),
				AttributeType: ddbtypes.ScalarAttributeTypeS,
			},
		},
		KeySchema: []ddbtypes.KeySchemaElement{
			{
				AttributeName: aws.String(hashKey),
				KeyType:       ddbtypes.KeyTypeHash,
			},
		},
	}
}

// EnsureDynamoDBTable verifies the named DynamoDB table exists.
// If not, creates it with PAY_PER_REQUEST billing and waits for it to become active.
func EnsureDynamoDBTable(ctx context.Context, cfg aws.Config, schema ddbTableSchema) error {
	ddbc := dynamodb.NewFromConfig(cfg)

	_, err := ddbc.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(schema.TableName),
	})
	if err == nil {
		utilities.LogProgress("infra", "dynamodb", "table exists", "table="+schema.TableName)
		return nil
	}

	if !isResourceNotFound(err) {
		return fmt.Errorf("describe table %q: %w", schema.TableName, err)
	}

	utilities.LogProgress("infra", "dynamodb", "creating table", "table="+schema.TableName)

	if _, err := ddbc.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:            aws.String(schema.TableName),
		AttributeDefinitions: schema.Attributes,
		KeySchema:            schema.KeySchema,
		BillingMode:          ddbtypes.BillingModePayPerRequest,
	}); err != nil {
		if isResourceInUse(err) {
			utilities.LogProgress("infra", "dynamodb", "table already creating", "table="+schema.TableName)
		} else {
			return fmt.Errorf("create table %q: %w", schema.TableName, err)
		}
	}

	waiter := dynamodb.NewTableExistsWaiter(ddbc)
	if err := waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(schema.TableName),
	}, 2*time.Minute); err != nil {
		return fmt.Errorf("wait for table %q: %w", schema.TableName, err)
	}

	utilities.LogProgress("infra", "dynamodb", "table active", "table="+schema.TableName)
	return nil
}

// isResourceNotFound checks if the error is a DynamoDB ResourceNotFoundException.
func isResourceNotFound(err error) bool {
	var rnf *ddbtypes.ResourceNotFoundException
	return errors.As(err, &rnf)
}

func isResourceInUse(err error) bool {
	var riu *ddbtypes.ResourceInUseException
	return errors.As(err, &riu)
}

// EnsureInfrastructure provisions all required AWS resources for the application.
// Idempotent — safe to call on every cold start.
//
// Resources created if missing:
//   - S3 bucket (from aws-config.yml)
//   - DynamoDB tables: user_identity and failed_records
//   - EventBridge custom event bus
func EnsureInfrastructure(ctx context.Context) error {
	if !Ready() {
		return fmt.Errorf("aws: not authenticated — call Init() first")
	}

	acct := GetAccount()
	cfg := acct.Config()
	awsCfg := config.AWS()

	utilities.LogProgress("infra", "ensure", "starting infrastructure check")

	if err := EnsureS3Bucket(ctx, cfg, awsCfg.S3.Bucket, awsCfg.Region); err != nil {
		return fmt.Errorf("s3 bucket: %w", err)
	}

	userTable := newSimpleTable(awsCfg.DynamoDB.UserIdentityTable, "document_id")
	if err := EnsureDynamoDBTable(ctx, cfg, userTable); err != nil {
		return fmt.Errorf("dynamodb user_identity table: %w", err)
	}

	failedTable := newSimpleTable(awsCfg.DynamoDB.FailedRecordsTable, "document_id")
	if err := EnsureDynamoDBTable(ctx, cfg, failedTable); err != nil {
		return fmt.Errorf("dynamodb failed_records table: %w", err)
	}

	if err := EnsureEventBridgeBus(ctx, cfg, awsCfg.EventBridge.BusName); err != nil {
		return fmt.Errorf("eventbridge bus: %w", err)
	}

	utilities.LogProgress("infra", "ensure", "infrastructure ready",
		"bucket="+awsCfg.S3.Bucket,
		"user_table="+awsCfg.DynamoDB.UserIdentityTable,
		"failed_table="+awsCfg.DynamoDB.FailedRecordsTable,
		"bus="+awsCfg.EventBridge.BusName,
	)
	return nil
}
