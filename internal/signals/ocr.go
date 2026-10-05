package signals

import (
	"context"
	"strings"

	"github.com/aidandevv/craig/internal/domain"
	"github.com/aidandevv/craig/internal/trace"
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
	return o.vision.evaluateImages(ctx, o.name, FeatureTextDetection, o.groups, listing,
		func(ctx context.Context, imageURL string, index, total int) (domain.SignalResult, error) {
			data, err := o.vision.annotate(ctx, imageURL, "TEXT_DETECTION")
			if err != nil {
				return domain.SignalResult{}, err
			}
			text := extractedText(data)
			trace.Log(ctx, "vision", "%s image %d/%d: Vision OCR completed (%d characters)", o.name, index, total, len([]rune(text)))
			result := apply(o.name, o.groups, strings.ToLower(text))
			result.CheckedAt = responseTime(data)
			_, result.Incomplete = firstResponse(data)["error"]
			if len(result.Flags) > 0 {
				result.Details = append(result.Details, "image text: "+truncate(text, 200))
			}
			return result, nil
		})
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
