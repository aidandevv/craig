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
	return r.vision.evaluateImages(ctx, r.name, FeatureWebDetection, r.groups, listing,
		func(ctx context.Context, imageURL string, index, total int) (domain.SignalResult, error) {
			data, err := r.vision.annotate(ctx, imageURL, "WEB_DETECTION")
			if err != nil {
				return domain.SignalResult{}, err
			}
			matches := matchingWebEvidence(data)
			urls := matches.urls()
			trace.Log(ctx, "vision", "%s image %d/%d: Vision returned %d matching page(s), %d full/partial image URL(s)", r.name, index, total, len(matches.pages), len(matches.images))
			result := apply(r.name, r.groups, strings.ToLower(strings.Join(urls, "\n")))
			result.ImageMatches = evidenceForMatches(r.groups, imageURL, matches)
			trace.Log(ctx, "vision", "%s image %d/%d: %d source rule(s) matched", r.name, index, total, len(result.Flags))
			result.CheckedAt = responseTime(data)
			_, result.Incomplete = firstResponse(data)["error"]
			if len(urls) > 0 {
				result.Details = append(result.Details, "matching pages: "+summarize(urls, 5))
			}
			return result, nil
		})
}

// Preserve page-specific image associations; global images have no known page.
type webEvidence struct {
	pages      []string
	images     []string
	pageImages map[string][]string
}

func (e webEvidence) urls() []string { return append(append([]string{}, e.pages...), e.images...) }
func imageURLsFrom(items any) []string {
	result := []string{}
	entries, _ := items.([]any)
	for _, item := range entries {
		entry, _ := item.(map[string]any)
		value, _ := entry["url"].(string)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
func matchingWebEvidence(data map[string]any) webEvidence {
	response := firstResponse(data)
	detection, _ := response["webDetection"].(map[string]any)
	result := webEvidence{pageImages: map[string][]string{}}
	result.pages = imageURLsFrom(detection["pagesWithMatchingImages"])
	result.images = append(imageURLsFrom(detection["fullMatchingImages"]), imageURLsFrom(detection["partialMatchingImages"])...)
	pages, _ := detection["pagesWithMatchingImages"].([]any)
	for _, item := range pages {
		page, _ := item.(map[string]any)
		pageURL, _ := page["url"].(string)
		images := append(imageURLsFrom(page["fullMatchingImages"]), imageURLsFrom(page["partialMatchingImages"])...)
		result.pageImages[pageURL] = images
		result.images = append(result.images, images...)
	}
	return result
}
func matchingPageURLs(data map[string]any) []string { return matchingWebEvidence(data).urls() }
func evidenceForMatches(groups []MatchGroup, listingImageURL string, matches webEvidence) []domain.ImageMatchEvidence {
	evidence := []domain.ImageMatchEvidence{}
	for _, group := range groups {
		for _, value := range matches.urls() {
			if _, ok := group.matches(strings.ToLower(value)); !ok {
				continue
			}
			pageURL, imageURL := value, ""
			if images, isPage := matches.pageImages[value]; isPage {
				if len(images) > 0 {
					imageURL = images[0]
				}
			} else {
				imageURL = value
				for _, page := range matches.pages {
					for _, image := range matches.pageImages[page] {
						if image == value {
							pageURL = page
							break
						}
					}
					if pageURL != value {
						break
					}
				}
			}
			evidence = append(evidence, domain.ImageMatchEvidence{Rule: group.Rule, ListingImageURL: listingImageURL, SourcePageURL: pageURL, SourceImageURL: imageURL})
			break
		}
	}
	return evidence
}

func summarize(values []string, max int) string {
	if len(values) <= max {
		return strings.Join(values, ", ")
	}
	return strings.Join(values[:max], ", ") + ", …"
}
