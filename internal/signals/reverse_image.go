package signals

import (
	"context"
	"strings"

	"github.com/aidandevv/craig-extension/internal/domain"
	"github.com/aidandevv/craig-extension/internal/trace"
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
	return r.vision.evaluateImages(ctx, r.name, FeatureWebDetection, true, r.groups, listing,
		func(ctx context.Context, imageURL string, index, total int) (domain.SignalResult, error) {
			data, err := r.vision.annotate(ctx, imageURL, "WEB_DETECTION")
			if err != nil {
				return domain.SignalResult{}, err
			}
			matches := matchingWebEvidence(data)
			urls := matches.urls()
			trace.Log(ctx, "vision", "%s image %d/%d: Vision found %d matching web page URL(s)", r.name, index, total, len(urls))
			result := apply(r.name, r.groups, strings.ToLower(strings.Join(urls, "\n")))
			result.ImageMatches = evidenceForMatches(r.groups, imageURL, matches)
			if len(urls) > 0 {
				result.Details = append(result.Details, "matching pages: "+summarize(urls, 5))
			}
			return result, nil
		})
}

// webEvidence retains the two forms that Vision returns: the page on which it
// found a matching image, and (when available) a matching image URL. Vision
// does not map each page to a particular image, so a preview is presented as
// supporting evidence rather than a claim that it came from that exact page.
type webEvidence struct {
	pages  []string
	images []string
}

func (e webEvidence) urls() []string { return append(append([]string{}, e.pages...), e.images...) }

// matchingWebEvidence flattens the relevant Web Detection response fields and
// keeps enough structure to render a source image beside the listing image.
func matchingWebEvidence(data map[string]any) webEvidence {
	response := firstResponse(data)
	if response == nil {
		return webEvidence{}
	}
	detection, _ := response["webDetection"].(map[string]any)
	if detection == nil {
		return webEvidence{}
	}
	result := webEvidence{}
	seen := map[string]bool{}
	collect := func(key string, target *[]string) {
		items, _ := detection[key].([]any)
		for _, item := range items {
			entry, _ := item.(map[string]any)
			if value, _ := entry["url"].(string); value != "" && !seen[value] {
				seen[value] = true
				*target = append(*target, value)
			}
		}
	}
	collect("pagesWithMatchingImages", &result.pages)
	collect("fullMatchingImages", &result.images)
	collect("partialMatchingImages", &result.images)
	return result
}

// matchingPageURLs remains a compact helper for callers and tests that only
// need the flattened provider URLs.
func matchingPageURLs(data map[string]any) []string { return matchingWebEvidence(data).urls() }

func evidenceForMatches(groups []MatchGroup, listingImageURL string, matches webEvidence) []domain.ImageMatchEvidence {
	urls := matches.urls()
	evidence := []domain.ImageMatchEvidence{}
	for _, group := range groups {
		matchedURL := ""
		for _, value := range urls {
			if _, ok := group.matches(strings.ToLower(value)); ok {
				matchedURL = value
				break
			}
		}
		if matchedURL == "" {
			continue
		}
		sourceImage := ""
		for _, value := range matches.images {
			if value == matchedURL {
				sourceImage = value
				break
			}
		}
		if sourceImage == "" && len(matches.images) > 0 {
			sourceImage = matches.images[0]
		}
		evidence = append(evidence, domain.ImageMatchEvidence{
			Rule:            group.Rule,
			ListingImageURL: listingImageURL,
			SourcePageURL:   matchedURL,
			SourceImageURL:  sourceImage,
		})
	}
	return evidence
}

func summarize(values []string, max int) string {
	if len(values) <= max {
		return strings.Join(values, ", ")
	}
	return strings.Join(values[:max], ", ") + ", …"
}
