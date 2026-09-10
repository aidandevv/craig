// Package domain holds the types shared across the analysis pipeline.
package domain

import "time"

// Listing is the normalized representation of a marketplace listing submitted
// for analysis. Marketplace-specific shape is normalized away before a listing
// reaches this type, so signals never need to know where it came from.
//
// Nothing here identifies the person browsing. The daemon analyzes a listing
// and forgets it; only image-signal results are cached, keyed by image hash.
type Listing struct {
	Marketplace string     `json:"marketplace"`
	URL         string     `json:"listing_url,omitempty"`
	Title       string     `json:"title"`
	Price       int        `json:"price,omitempty"`
	Currency    string     `json:"currency,omitempty"`
	Description string     `json:"description,omitempty"`
	Images      []string   `json:"images,omitempty"`
	Captions    []string   `json:"captions,omitempty"`
	Contact     Contact    `json:"contact"`
	PostedAt    *time.Time `json:"posted_at,omitempty"`
}

// Contact records how a seller can be reached. The booleans describe
// affordances the extension observed on the page rather than parsed text.
type Contact struct {
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	RelayOnly bool   `json:"relay_only,omitempty"`
	AppOnly   bool   `json:"app_only,omitempty"`
}

// Reasons a signal could not run. This set is closed so the UI can phrase each
// one, and so a listing that could not be checked never renders like a clean one.
const (
	SkipNoAPIKey        = "no_api_key"
	SkipBudgetExhausted = "vision_budget_exhausted"
	SkipNoImages        = "no_images"
	SkipMissingField    = "missing_field"
	SkipProviderError   = "provider_error"
)

// SignalResult is one detector's verdict. Risk is a calibrated contribution to
// the composite score, never a probability and never a claim of fraud.
//
// Flags carry the names of the rules that matched, which is how a detector that
// evaluates several rules in one pass attributes its findings back to them.
type SignalResult struct {
	Name         string               `json:"name"`
	Risk         float64              `json:"risk"`
	Flags        []string             `json:"flags,omitempty"`
	Hard         bool                 `json:"hard"`
	Details      []string             `json:"details,omitempty"`
	ImageMatches []ImageMatchEvidence `json:"image_matches,omitempty"`
	Skipped      string               `json:"skipped,omitempty"`
	LatencyMS    int64                `json:"latency_ms"`
}

// ImageMatchEvidence is a provider-returned reverse-image match attributed to
// one rule. It is kept on the signal result so cached Vision responses retain
// the evidence needed to explain a later assessment.
type ImageMatchEvidence struct {
	Rule            string `json:"rule"`
	ListingImageURL string `json:"listing_image_url"`
	SourcePageURL   string `json:"source_page_url"`
	SourceImageURL  string `json:"source_image_url,omitempty"`
}
