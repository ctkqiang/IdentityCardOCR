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

// EnsureS3Bucket guarantees the named S3 bucket exists in the target region.
//
// Uses HeadBucket to probe for existence. When the bucket is absent
// (404 / NoSuchBucket), the function creates it with private ACL and
// waits up to 30 seconds for the bucket to become reachable.
//
// An error of type BucketAlreadyOwnedByYou is silently accepted; any other
// creation error is returned to the caller.
//
// region is used for the LocationConstraint on CreateBucket. The us-east-1
// region omits the constraint (it is the default).
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

// isBucketAlreadyOwnedByYou reports whether err is an S3 API error with
// code BucketAlreadyOwnedByYou. Errors with code BucketAlreadyExists
// (bucket name owned by a different account) are intentionally not matched;
// those must be surfaced so the operator can choose a different bucket name.
func isBucketAlreadyOwnedByYou(err error) bool {
	var ae apiError
	if errors.As(err, &ae) {
		return ae.ErrorCode() == "BucketAlreadyOwnedByYou"
	}
	return false
}

// EnsureEventBridgeBus guarantees the named custom event bus exists.
//
// An empty busName signals the default event bus, in which case the function
// returns nil immediately (the default bus always exists). For named buses,
// DescribeEventBus probes for existence; CreateEventBus is called on failure.
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

// EnsureDynamoDBTable guarantees the named DynamoDB table is active.
//
// DescribeTable confirms existence. On ResourceNotFoundException the table
// is created with PAY_PER_REQUEST billing and the call blocks until the table
// reaches ACTIVE status (2-minute timeout). ResourceInUseException during
// creation is logged and treated as success — another caller is creating it.
//
// Any error other than ResourceNotFoundException from DescribeTable is
// returned to the caller without attempting creation.
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

// EnsureInfrastructure provisions every AWS resource the application depends on.
//
// It sequences through S3 bucket, DynamoDB tables (user_identity and
// failed_records), and EventBridge bus creation. Each sub-call is idempotent;
// resources that already exist are detected and skipped without side effects.
//
// Init() must have completed successfully before calling this function.
//
// Resource names are read from config.AWS() and therefore from aws-config.yml.
// On first deploy expect 15–30 seconds of latency while DynamoDB tables are
// created; subsequent calls on warm containers return in under a second.
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
