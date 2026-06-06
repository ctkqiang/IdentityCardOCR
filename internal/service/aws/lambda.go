package aws

import (
	"context"
	"identity_card_ocr/internal/event"
	"identity_card_ocr/internal/model"
)

func LambdaHandler(ctx context.Context, document *model.DocumentInfo, event *event.LambdaEvent) (model.Response, error) {
	return model.Response{
		Text:     event.Name,
		Document: *document,
		Err:      nil,
	}, nil
}
