package signals

import (
	"context"
	"strings"

	"github.com/aidandevv/craig-extension/internal/domain"
)

// OCR reads text baked into a listing photo. Watermarks are the target: an MLS
// or agency watermark on a supposedly private rental means the photo was taken
// from a real listing elsewhere.
type OCR struct {
	vision *Vision
	name   string
	groups []MatchGroup
}

type OCROptions struct {
	Name   string
	Groups []MatchGroup
}

func NewOCR(v *Vision, opts OCROptions) *OCR {
	name := opts.Name
	if name == "" {
		name = "ocr"
	}
	return &OCR{vision: v, name: name, groups: opts.Groups}
}

func (o *OCR) Name() string { return o.name }

func (o *OCR) Evaluate(ctx context.Context, listing domain.Listing) (domain.SignalResult, error) {
	pre, hash, proceed := o.vision.call(ctx, o.name, FeatureTextDetection, listing)
	if !proceed {
		return pre, nil
	}

	data, err := o.vision.annotate(ctx, listing.Images[0], "TEXT_DETECTION")
	if err != nil {
		return domain.SignalResult{
			Name:    o.name,
			Skipped: domain.SkipProviderError,
			Details: []string{err.Error()},
		}, nil
	}

	text := extractedText(data)
	result := apply(o.name, o.groups, strings.ToLower(text))
	if len(result.Flags) > 0 {
		result.Details = append(result.Details, "image text: "+truncate(text, 200))
	}
	return result, o.vision.store.Put(ctx, hash, o.name, result)
}

func extractedText(data map[string]any) string {
	response := firstResponse(data)
	if response == nil {
		return ""
	}
	full, _ := response["fullTextAnnotation"].(map[string]any)
	value, _ := full["text"].(string)
	return value
}

func truncate(value string, max int) string {
	runes := []rune(strings.Join(strings.Fields(value), " "))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "…"
}
