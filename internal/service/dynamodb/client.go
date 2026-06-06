package dynamodb

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// UserIdentity represents a successfully processed identity document stored in DynamoDB.
type UserIdentity struct {
	DocumentID  string `dynamodbav:"document_id"`
	IDNumber    string `dynamodbav:"id_number"`
	Name        string `dynamodbav:"name"`
	DateOfBirth string `dynamodbav:"date_of_birth"`
	Sex         string `dynamodbav:"sex"`
	Nationality string `dynamodbav:"nationality,omitempty"`
	ExpiryDate  string `dynamodbav:"expiry_date,omitempty"`
	RawText     string `dynamodbav:"raw_text,omitempty"`
	Country     string `dynamodbav:"country"`
	CreatedAt   string `dynamodbav:"created_at"`
}

// FailedRecord represents a failed OCR attempt stored in DynamoDB.
type FailedRecord struct {
	DocumentID string `dynamodbav:"document_id"`
	Error      string `dynamodbav:"error"`
	Phase      string `dynamodbav:"phase"`
	Country    string `dynamodbav:"country"`
	CreatedAt  string `dynamodbav:"created_at"`
}

// Client wraps the DynamoDB service client.
type Client struct {
	client *dynamodb.Client
}

// NewClient returns a DynamoDB service wrapper backed by the shared AWS SDK
// configuration. The returned Client is safe for concurrent use.
func NewClient(cfg aws.Config) *Client {
	return &Client{client: dynamodb.NewFromConfig(cfg)}
}

// PutUserIdentity writes a successfully processed identity document to DynamoDB.
//
// The item's CreatedAt field is set to the current UTC time if left empty.
// The table must exist before calling this method; no table creation is performed.
func (c *Client) PutUserIdentity(ctx context.Context, table string, item UserIdentity) error {
	if item.CreatedAt == "" {
		item.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("marshal user identity: %w", err)
	}

	_, err = c.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item:      av,
	})
	if err != nil {
		return fmt.Errorf("put user identity: %w", err)
	}
	return nil
}

// PutFailedRecord writes a failed OCR attempt to DynamoDB.
//
// The item's CreatedAt field is set to the current UTC time if left empty.
// Each call creates a new item; repeated failures for the same document_id
// overwrite the previous entry.
func (c *Client) PutFailedRecord(ctx context.Context, table string, item FailedRecord) error {
	if item.CreatedAt == "" {
		item.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("marshal failed record: %w", err)
	}

	_, err = c.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item:      av,
	})
	if err != nil {
		return fmt.Errorf("put failed record: %w", err)
	}
	return nil
}

// GetUserIdentity retrieves a user identity record by its document_id key.
//
// Returns nil, nil when no item matches the document ID.
// Returns an error only on DynamoDB service failures, not on missing items.
func (c *Client) GetUserIdentity(ctx context.Context, table, documentID string) (*UserIdentity, error) {
	out, err := c.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]ddbtypes.AttributeValue{
			"document_id": &ddbtypes.AttributeValueMemberS{Value: documentID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get user identity: %w", err)
	}
	if out.Item == nil {
		return nil, nil
	}

	var item UserIdentity
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return nil, fmt.Errorf("unmarshal user identity: %w", err)
	}
	return &item, nil
}
