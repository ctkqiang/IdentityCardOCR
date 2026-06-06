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

// NewBusClient returns an EventBridge publishing wrapper backed by the shared
// SDK configuration. When busName is empty, Put() targets the default event bus.
// The source parameter is written to every event's Source field.
func NewBusClient(cfg aws.Config, busName, source string) *BusClient {
	return &BusClient{
		client:  eventbridge.NewFromConfig(cfg),
		busName: busName,
		source:  source,
	}
}

// Put publishes a single event to EventBridge. Errors from the EventBridge
// API are silently discarded; the S3 event store is the durable source of truth.
// Callers must not depend on Put for delivery guarantees.
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

// PutBatch publishes events in a single PutEvents API call, capped at 10
// entries (the EventBridge service limit). Excess events beyond the 10th
// are silently dropped. As with Put, errors are discarded.
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

// EnsureBus calls DescribeEventBus to confirm the configured bus exists.
// Returns nil for the default bus (empty busName). Returns an error if the
// named bus cannot be described.
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
