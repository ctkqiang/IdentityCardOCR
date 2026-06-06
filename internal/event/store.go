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

// NewStore creates an S3-backed event Store using the shared SDK configuration.
// The prefix must not have a trailing slash; it is stripped if present.
func NewStore(cfg aws.Config, bucket, prefix string) *Store {
	return &Store{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
		prefix: strings.TrimRight(prefix, "/"),
	}
}

// Append writes an event as an immutable JSON object to S3.
//
// The object key is {prefix}/events/{documentID}/{timestamp_nano_20d}.json
// where the timestamp is the event's Timestamp field formatted as a 20-digit
// nanosecond Unix epoch. Nanosecond granularity prevents key collisions for
// events on the same document.
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

// Replay reads every event persisted for documentID, sorted by timestamp
// ascending. Returns an empty slice, not nil, when no events exist.
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

// ListDocuments returns every document ID with at least one persisted event.
// IDs are sorted alphabetically. Returns an empty slice when the event store
// contains no documents.
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
