package pipeline

import (
	"context"
	"fmt"

	"identity_card_ocr/internal/config"
	"identity_card_ocr/internal/event"
	"identity_card_ocr/internal/service"
	"identity_card_ocr/internal/utilities"
)

// Handler processes an event and returns zero or more resulting events.
type Handler func(ctx context.Context, evt event.Event) ([]event.Event, error)

// Pipeline is an event-driven OCR processing pipeline.
// Handlers are registered per event type and run sequentially when Process is called.
type Pipeline struct {
	handlers map[event.EventType][]Handler
	store    *event.Store
	bus      *event.BusClient
}

// New creates a Pipeline that persists and publishes events through store
// and bus. The default DocumentSubmitted handler is registered automatically.
func New(store *event.Store, bus *event.BusClient) *Pipeline {
	p := &Pipeline{
		handlers: make(map[event.EventType][]Handler),
		store:    store,
		bus:      bus,
	}
	p.Handle(event.DocumentSubmitted, p.handleDocumentSubmitted)
	return p
}

// Handle registers h as a handler for events of the given type. Multiple
// handlers for the same type run in registration order when Process is called.
func (p *Pipeline) Handle(typ event.EventType, h Handler) {
	p.handlers[typ] = append(p.handlers[typ], h)
}

// Process persists evt to the S3 event store, publishes it to EventBridge,
// and executes every handler registered for evt.Type. Events produced by
// handlers are recursively persisted and published.
//
// A handler error does not abort the pipeline; remaining handlers for the
// same event type still execute. Handler errors are logged but not returned.
// Only an S3 persistence failure on the trigger event causes Process to
// return an error.
func (p *Pipeline) Process(ctx context.Context, evt event.Event) ([]event.Event, error) {
	if err := p.store.Append(ctx, evt); err != nil {
		utilities.LogError("pipeline", "process", err, 0, "event_id="+evt.ID)
		return nil, fmt.Errorf("persist event: %w", err)
	}
	p.bus.Put(ctx, evt)

	var allResults []event.Event
	for _, h := range p.handlers[evt.Type] {
		results, err := h(ctx, evt)
		if err != nil {
			utilities.LogError("pipeline", "handler", err, 0,
				"event_type="+string(evt.Type), "event_id="+evt.ID)
			continue
		}
		for _, result := range results {
			if storeErr := p.store.Append(ctx, result); storeErr != nil {
				utilities.LogError("pipeline", "persist-result", storeErr, 0,
					"event_id="+result.ID)
			}
			p.bus.Put(ctx, result)
			allResults = append(allResults, result)
		}
	}
	return allResults, nil
}

// handleDocumentSubmitted processes a document.submitted event by running
// OCR extraction and parsing, then emitting a processing.completed or
// processing.failed event.
func (p *Pipeline) handleDocumentSubmitted(ctx context.Context, evt event.Event) ([]event.Event, error) {
	var payload event.DocumentSubmittedPayload
	if err := evt.UnmarshalDetail(&payload); err != nil {
		failed := event.New(event.ProcessingFailed, evt.DocumentID, event.ProcessingFailedPayload{
			Error: fmt.Sprintf("unmarshal payload: %v", err),
			Phase: "init",
		})
		return []event.Event{failed}, nil
	}

	imagePath := payload.ImagePath
	if imagePath == "" && payload.S3Bucket != "" {
		imagePath = payload.S3Key
	}

	var country config.Country
	switch payload.Country {
	case "china":
		country = config.CHINA
	case "malaysia":
		country = config.MALAYSIA
	case "us":
		country = config.US
	default:
		failed := event.New(event.ProcessingFailed, evt.DocumentID, event.ProcessingFailedPayload{
			Error: fmt.Sprintf("unknown country: %s", payload.Country),
			Phase: "init",
		})
		return []event.Event{failed}, nil
	}

	doc, err := service.ExtractTextFromIdentityDocument(imagePath, country)
	if err != nil {
		failed := event.New(event.ProcessingFailed, evt.DocumentID, event.ProcessingFailedPayload{
			Error: err.Error(),
			Phase: "ocr",
		})
		return []event.Event{failed}, nil
	}

	completed := event.New(event.ProcessingCompleted, evt.DocumentID, event.ProcessingCompletedPayload{
		IDNumber:    doc.IDNumber,
		Name:        doc.Name,
		Nationality: doc.Nationality,
		DateOfBirth: doc.DateOfBirth,
		Sex:         doc.Sex,
		ExpiryDate:  doc.ExpiryDate,
		RawText:     doc.RawText,
	})
	return []event.Event{completed}, nil
}
