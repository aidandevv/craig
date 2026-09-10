package signals

import (
	"context"
	"strings"

	"github.com/aidandevv/craig-extension/internal/domain"
)

// ReverseImage asks Vision where else on the web a listing's photo appears.
// A rental photo that is also on Zillow or a stock library is one of the
// strongest single indicators available, which is why it carries heavy weight.
//
// One instance serves every reverse-search rule in the ruleset: the groups are
// evaluated against a single API response, so N rules cost one billable unit.
type ReverseImage struct {
	vision *Vision
	name   string
	groups []MatchGroup
}

type ReverseImageOptions struct {
	Name   string // signal name; also the cache key namespace
	Groups []MatchGroup
}

func NewReverseImage(v *Vision, opts ReverseImageOptions) *ReverseImage {
	name := opts.Name
	if name == "" {
		name = "reverse_image"
	}
	return &ReverseImage{vision: v, name: name, groups: opts.Groups}
}

func (r *ReverseImage) Name() string { return r.name }

func (r *ReverseImage) Evaluate(ctx context.Context, listing domain.Listing) (domain.SignalResult, error) {
	pre, hash, proceed := r.vision.call(ctx, r.name, FeatureWebDetection, listing)
	if !proceed {
		return pre, nil
	}

	data, err := r.vision.annotate(ctx, listing.Images[0], "WEB_DETECTION")
	if err != nil {
		return domain.SignalResult{
			Name:    r.name,
			Skipped: domain.SkipProviderError,
			Details: []string{err.Error()},
		}, nil
	}

	urls := matchingPageURLs(data)
	result := apply(r.name, r.groups, strings.ToLower(strings.Join(urls, "\n")))
	if len(urls) > 0 {
		result.Details = append(result.Details, "matching pages: "+summarize(urls, 5))
	}
	return result, r.vision.store.Put(ctx, hash, r.name, result)
}

// matchingPageURLs flattens every kind of web-detection match into one list.
func matchingPageURLs(data map[string]any) []string {
	response := firstResponse(data)
	if response == nil {
		return nil
	}
	detection, _ := response["webDetection"].(map[string]any)
	if detection == nil {
		return nil
	}
	var urls []string
	for _, key := range []string{"pagesWithMatchingImages", "fullMatchingImages", "partialMatchingImages"} {
		items, _ := detection[key].([]any)
		for _, item := range items {
			entry, _ := item.(map[string]any)
			if value, _ := entry["url"].(string); value != "" {
				urls = append(urls, value)
			}
		}
	}
	return urls
}

func summarize(values []string, max int) string {
	if len(values) <= max {
		return strings.Join(values, ", ")
	}
	return strings.Join(values[:max], ", ") + ", …"
}
