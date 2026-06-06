package lambda

import (
	"context"
	"fmt"
	"identity_card_ocr/internal/config"
	"identity_card_ocr/internal/event"
	"identity_card_ocr/internal/pipeline"
	"identity_card_ocr/internal/service"
	"identity_card_ocr/internal/service/dynamodb"
	"identity_card_ocr/internal/utilities"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	awsaccount "identity_card_ocr/internal/service/aws"
)

// Response is the Lambda invocation result.
type Response struct {
	Message string `json:"message"`
	Total   int    `json:"total"`
	Passed  int    `json:"passed"`
	Failed  int    `json:"failed"`
}

// HandleRequest is the Lambda entry point. It processes an S3 event,
// runs OCR on each object, emits events, and stores results in DynamoDB.
func HandleRequest(ctx context.Context, s3Event events.S3Event) (Response, error) {
	utilities.LogProgress("lambda", "handler", "invoked", fmt.Sprintf("records=%d", len(s3Event.Records)))

	if len(s3Event.Records) == 0 {
		utilities.LogProgress("lambda", "handler", "no objects to process")
		return Response{Message: "no objects to process"}, nil
	}

	if !awsaccount.Ready() {
		if err := awsaccount.Init(ctx); err != nil {
			return Response{}, fmt.Errorf("aws init: %w", err)
		}
	}

	cfg := config.AWS()
	s3Client := s3.NewFromConfig(awsaccount.GetAccount().Config())
	store := event.NewStore(awsaccount.GetAccount().Config(), cfg.S3.Bucket, cfg.S3.Path)
	bus := event.NewBusClient(awsaccount.GetAccount().Config(), cfg.EventBridge.BusName, cfg.EventBridge.Source)
	ddb := dynamodb.NewClient(awsaccount.GetAccount().Config())
	pipe := pipeline.New(store, bus)

	var passed, failed int

	for _, record := range s3Event.Records {
		bucket := record.S3.Bucket.Name
		key := record.S3.Object.Key
		docID := key

		utilities.LogProgress("lambda", "handler", "processing", "bucket="+bucket, "key="+key)

		country, err := inferCountry(key)
		if err != nil {
			failed++
			emitFailure(ctx, pipe, ddb, cfg, docID, err.Error(), "init", "")
			continue
		}

		imagePath := filepath.Join("/tmp", filepath.Base(key))
		if err := downloadS3Object(ctx, s3Client, bucket, key, imagePath); err != nil {
			failed++
			emitFailure(ctx, pipe, ddb, cfg, docID, fmt.Sprintf("s3 download: %v", err), "init", country.String())
			continue
		}

		doc, err := service.ExtractTextFromIdentityDocument(imagePath, country)
		os.Remove(imagePath) // clean up temp file

		if err != nil {
			failed++
			emitFailure(ctx, pipe, ddb, cfg, docID, err.Error(), "ocr", country.String())
			continue
		}

		passed++
		completed := event.New(event.ProcessingCompleted, docID, event.ProcessingCompletedPayload{
			IDNumber:    doc.IDNumber,
			Name:        doc.Name,
			Nationality: doc.Nationality,
			DateOfBirth: doc.DateOfBirth,
			Sex:         doc.Sex,
			ExpiryDate:  doc.ExpiryDate,
			RawText:     doc.RawText,
		})

		if _, err := pipe.Process(ctx, completed); err != nil {
			utilities.LogError("lambda", "pipeline-process", err, 0, "doc_id="+docID)
		}

		if err := ddb.PutUserIdentity(ctx, cfg.DynamoDB.UserIdentityTable, dynamodb.UserIdentity{
			DocumentID:  docID,
			IDNumber:    doc.IDNumber,
			Name:        doc.Name,
			DateOfBirth: doc.DateOfBirth,
			Sex:         doc.Sex,
			Nationality: doc.Nationality,
			ExpiryDate:  doc.ExpiryDate,
			RawText:     doc.RawText,
			Country:     country.String(),
		}); err != nil {
			utilities.LogError("lambda", "dynamodb-put", err, 0, "doc_id="+docID)
		}
	}

	return Response{
		Message: fmt.Sprintf("processed %d objects", len(s3Event.Records)),
		Total:   len(s3Event.Records),
		Passed:  passed,
		Failed:  failed,
	}, nil
}

func downloadS3Object(ctx context.Context, client *s3.Client, bucket, key, dest string) error {
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer out.Body.Close()

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, out.Body)
	return err
}

func emitFailure(ctx context.Context, pipe *pipeline.Pipeline, ddb *dynamodb.Client, cfg config.AWSConfig, docID, errMsg, phase, country string) {
	failed := event.New(event.ProcessingFailed, docID, event.ProcessingFailedPayload{
		Error: errMsg,
		Phase: phase,
	})

	if _, err := pipe.Process(ctx, failed); err != nil {
		utilities.LogError("lambda", "pipeline-failed-event", err, 0, "doc_id="+docID)
	}

	if err := ddb.PutFailedRecord(ctx, cfg.DynamoDB.FailedRecordsTable, dynamodb.FailedRecord{
		DocumentID: docID,
		Error:      errMsg,
		Phase:      phase,
		Country:    country,
	}); err != nil {
		utilities.LogError("lambda", "dynamodb-failed-put", err, 0, "doc_id="+docID)
	}
}

// inferCountry determines the country from the S3 key prefix.
// Expected prefix format: "china/...", "malaysia/...", "us/..."
func inferCountry(key string) (config.Country, error) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 0 {
		return -1, fmt.Errorf("cannot infer country from key: %s", key)
	}
	return config.CountryFromString(strings.ToLower(parts[0]))
}
