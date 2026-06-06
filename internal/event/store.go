package event

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Store is an S3-backed append-only event log.
// Each event is stored as an immutable JSON object keyed by document ID and nanosecond timestamp.
type Store struct {
	client *s3.Client
	bucket string
	prefix string
}

// NewStore creates an event Store using the given S3 client, bucket, and key prefix.
func NewStore(cfg aws.Config, bucket, prefix string) *Store {
	return &Store{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
		prefix: strings.TrimRight(prefix, "/"),
	}
}

// Append writes a single event as a JSON object to S3.
// Key: {prefix}/events/{documentID}/{timestamp_nano_20d}.json
func (s *Store) Append(ctx context.Context, evt Event) error {
	body, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("event marshal: %w", err)
	}

	ts := fmt.Sprintf("%020d", evt.Timestamp.UnixNano())
	key := fmt.Sprintf("%s/events/%s/%s.json", s.prefix, evt.DocumentID, ts)

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("s3 put event: %w", err)
	}
	return nil
}

// Replay reads all events for a document ID in chronological order.
func (s *Store) Replay(ctx context.Context, documentID string) ([]Event, error) {
	prefix := fmt.Sprintf("%s/events/%s/", s.prefix, documentID)

	var events []Event
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("s3 list events: %w", err)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(s.bucket),
				Key:    obj.Key,
			})
			if err != nil {
				return nil, fmt.Errorf("s3 get event %s: %w", *obj.Key, err)
			}
			var evt Event
			if err := json.NewDecoder(out.Body).Decode(&evt); err != nil {
				out.Body.Close()
				return nil, fmt.Errorf("decode event %s: %w", *obj.Key, err)
			}
			out.Body.Close()
			events = append(events, evt)
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
	return events, nil
}

// ListDocuments returns all document IDs that have at least one event.
func (s *Store) ListDocuments(ctx context.Context) ([]string, error) {
	prefix := fmt.Sprintf("%s/events/", s.prefix)

	seen := make(map[string]bool)
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("s3 list documents: %w", err)
		}
		for _, cp := range page.CommonPrefixes {
			if cp.Prefix == nil {
				continue
			}
			p := strings.TrimPrefix(*cp.Prefix, prefix)
			p = strings.TrimRight(p, "/")
			if p != "" {
				seen[p] = true
			}
		}
	}

	docs := make([]string, 0, len(seen))
	for d := range seen {
		docs = append(docs, d)
	}
	sort.Strings(docs)
	return docs, nil
}
