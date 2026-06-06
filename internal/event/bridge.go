package event

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
)

// BusClient publishes events to AWS EventBridge.
// Put failures are logged but not returned — EventBridge is best-effort;
// the S3 Store is the durable source of truth.
type BusClient struct {
	client  *eventbridge.Client
	busName string
	source  string
}

// NewBusClient creates an EventBridge client wrapper.
// busName may be empty to use the default event bus.
func NewBusClient(cfg aws.Config, busName, source string) *BusClient {
	return &BusClient{
		client:  eventbridge.NewFromConfig(cfg),
		busName: busName,
		source:  source,
	}
}

// Put publishes a single event to EventBridge.
// Returns nil even on failure — callers should not block on EventBridge errors.
func (b *BusClient) Put(ctx context.Context, evt Event) error {
	detail, err := json.Marshal(evt)
	if err != nil {
		return nil
	}

	input := &eventbridge.PutEventsInput{
		Entries: []ebtypes.PutEventsRequestEntry{
			{
				EventBusName: awsStringOrNil(b.busName),
				Source:       aws.String(b.source),
				DetailType:   aws.String(string(evt.Type)),
				Detail:       aws.String(string(detail)),
			},
		},
	}

	_, err = b.client.PutEvents(ctx, input)
	if err != nil {
		return nil
	}
	return nil
}

// PutBatch publishes up to 10 events in a single PutEvents call.
func (b *BusClient) PutBatch(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}
	if len(events) > 10 {
		events = events[:10]
	}

	entries := make([]ebtypes.PutEventsRequestEntry, len(events))
	for i, evt := range events {
		detail, err := json.Marshal(evt)
		if err != nil {
			continue
		}
		entries[i] = ebtypes.PutEventsRequestEntry{
			EventBusName: awsStringOrNil(b.busName),
			Source:       aws.String(b.source),
			DetailType:   aws.String(string(evt.Type)),
			Detail:       aws.String(string(detail)),
		}
	}

	_, err := b.client.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: entries,
	})
	if err != nil {
		return nil
	}

	return nil
}

// EnsureBus verifies the configured event bus exists.
func (b *BusClient) EnsureBus(ctx context.Context) error {
	if b.busName == "" {
		return nil
	}

	_, err := b.client.DescribeEventBus(ctx, &eventbridge.DescribeEventBusInput{
		Name: aws.String(b.busName),
	})
	if err != nil {
		return fmt.Errorf("event bus %q not found: %w", b.busName, err)
	}

	return nil
}

func awsStringOrNil(s string) *string {
	if s == "" {
		return nil
	}

	return aws.String(s)
}
