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

// NewClient creates a DynamoDB client from the shared AWS config.
func NewClient(cfg aws.Config) *Client {
	return &Client{client: dynamodb.NewFromConfig(cfg)}
}

// PutUserIdentity inserts a successfully processed identity into the given table.
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

// PutFailedRecord inserts a failed OCR attempt into the given table.
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

// GetUserIdentity retrieves a user identity by document ID.
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
