// Package signals holds detectors that call external providers. Everything here
// is optional: with no credentials configured each signal reports that it could
// not run, and the rest of the analysis proceeds.
package signals

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/aidandevv/craig/internal/detect"
	"github.com/aidandevv/craig/internal/domain"
	"github.com/aidandevv/craig/internal/trace"
)

// Vision bills each feature separately, so each carries its own budget line.
const (
	FeatureWebDetection  = "web_detection"
	FeatureTextDetection = "text_detection"
)

const defaultEndpoint = "https://vision.googleapis.com/v1/images:annotate"

const craigslistImageHost = "images.craigslist.org"

// Meter records provider usage. A daemon can wire this to real counters; tests
// and the bare-bones path use NopMeter.
type Meter interface {
	VisionUnitUsed(feature string)
	VisionUnitSkipped(feature string)
}

// NopMeter discards usage events.
type NopMeter struct{}

func (NopMeter) VisionUnitUsed(string)    {}
func (NopMeter) VisionUnitSkipped(string) {}

// EvidenceStore persists provider evidence and the monthly unit ledger. The
// daemon backs it with SQLite; the browser build backs it with extension
// storage. Implementations must be safe for concurrent use.
type EvidenceStore interface {
	GetEvidence(ctx context.Context, hash, feature string) (map[string]any, time.Time, bool)
	PutEvidence(ctx context.Context, hash, feature string, data map[string]any, at time.Time) error
	ReserveVisionUnit(ctx context.Context, feature string, monthlyCap int) (bool, error)
}

// Options configures the Vision client. Credentials come from the caller, never
// from the environment directly, so the config file stays the single source of
// truth for what this tool is allowed to talk to.
type Options struct {
	Endpoint   string        // defaults to the public Vision endpoint
	APIKey     string        // restricted API key; takes precedence over ADC
	UseADC     bool          // fall back to application-default credentials
	MonthlyCap int           // per-feature unit ceiling
	Timeout    time.Duration // per-request timeout
}

// Vision is a thin REST client. REST rather than the SDK keeps it testable
// against an httptest server and keeps the dependency surface small.
type Vision struct {
	store    EvidenceStore
	meter    Meter
	endpoint string
	apiKey   string
	enabled  bool
	cap      int
	client   *http.Client
}

// NewVision builds a client. It never fails on missing credentials: an
// unconfigured Vision is a disabled Vision, which is a supported mode.
func NewVision(store EvidenceStore, meter Meter, opts Options) *Vision {
	if meter == nil {
		meter = NopMeter{}
	}
	endpoint := strings.TrimRight(opts.Endpoint, "/")
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	v := &Vision{
		store:    store,
		meter:    meter,
		endpoint: endpoint,
		apiKey:   opts.APIKey,
		cap:      opts.MonthlyCap,
		client:   &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
	switch {
	case opts.APIKey != "":
		v.enabled = true
	case opts.UseADC:
		creds, err := google.FindDefaultCredentials(context.Background(),
			"https://www.googleapis.com/auth/cloud-platform")
		if err == nil {
			client := oauth2.NewClient(context.Background(), creds.TokenSource)
			client.Timeout = timeout
			v.client = client
			v.enabled = true
		}
	}
	return v
}

// Enabled reports whether credentials were configured.
func (v *Vision) Enabled() bool { return v.enabled }

// MatchGroup ties a set of substrings to the rule that owns them. One provider
// call can satisfy many rules; the group is how a hit is attributed back to the
// rule the user actually wrote.
type MatchGroup struct {
	Rule   string
	Tokens []string
	Weight float64
	Hard   bool
}

func (g MatchGroup) matches(haystack string) (string, bool) {
	for _, token := range g.Tokens {
		token = strings.ToLower(strings.TrimSpace(token))
		if token != "" && strings.Contains(haystack, token) {
			return token, true
		}
	}
	return "", false
}

// apply scores a haystack against every group, returning the combined result.
// Several rules may share one API call, so risk is the sum of those that hit.
func apply(name string, groups []MatchGroup, haystack string) domain.SignalResult {
	result := domain.SignalResult{Name: name}
	for _, group := range groups {
		token, ok := group.matches(haystack)
		if !ok {
			continue
		}
		result.Risk += group.Weight
		result.Flags = append(result.Flags, group.Rule)
		result.Details = append(result.Details,
			fmt.Sprintf("%s: matched %q", group.Rule, token))
		if group.Hard {
			result.Hard = true
		}
	}
	result.Risk = detect.Clamp(result.Risk)
	return result
}

type imageOutcome struct {
	index  int
	result domain.SignalResult
}

// evaluateImages runs one Vision feature against every listing image. Cache
// hits are re-scored against current rules; newly processed images each
// reserve one unit because Vision bills per feature per image. A signal is
// skipped only when no image could be evaluated, so a provider failure on one
// photo cannot erase evidence found in another.
func (v *Vision) evaluateImages(
	ctx context.Context,
	name, feature string,
	groups []MatchGroup,
	listing domain.Listing,
	evaluate func(context.Context, string, int, int) (domain.SignalResult, error),
) (domain.SignalResult, error) {
	if len(listing.Images) == 0 {
		trace.Log(ctx, "vision", "%s skipped: listing has no images", name)
		return domain.SignalResult{Name: name, Skipped: domain.SkipNoImages}, nil
	}
	if !v.enabled {
		trace.Log(ctx, "vision", "%s skipped: Google Vision credentials are not configured", name)
		return domain.SignalResult{Name: name, Skipped: domain.SkipNoAPIKey}, nil
	}

	total := len(listing.Images)
	trace.Log(ctx, "vision", "%s will inspect %d listing image(s) with %s", name, total, feature)
	outcomes := make([]imageOutcome, 0, total)
	for index, imageURL := range listing.Images {
		position := index + 1
		if ctx.Err() != nil {
			outcomes = append(outcomes, imageOutcome{index: position, result: domain.SignalResult{Name: name, Skipped: domain.SkipProviderError}})
			continue
		}
		if err := validateImageURL(imageURL); err != nil {
			trace.Log(ctx, "vision", "%s image %d/%d skipped: %s", name, position, total, err)
			outcomes = append(outcomes, imageOutcome{index: position, result: domain.SignalResult{Name: name, Skipped: domain.SkipMissingField}})
			continue
		}

		result, err := evaluate(ctx, imageURL, position, total)
		if err != nil {
			trace.Log(ctx, "vision", "%s image %d/%d skipped: %s", name, position, total, skipReason(err))
			outcomes = append(outcomes, imageOutcome{index: position, result: domain.SignalResult{Name: name, Skipped: skipReason(err)}})
			continue
		}
		result.Name = name
		outcomes = append(outcomes, imageOutcome{index: position, result: result})
	}
	return mergeImageOutcomes(name, groups, outcomes), nil
}

func mergeImageOutcomes(name string, groups []MatchGroup, outcomes []imageOutcome) domain.SignalResult {
	weights := make(map[string]MatchGroup, len(groups))
	for _, group := range groups {
		weights[group.Rule] = group
	}
	merged := domain.SignalResult{Name: name, ImagesTotal: len(outcomes)}
	matched := make(map[string]bool)
	seenEvidence := make(map[string]bool)
	evaluated := 0
	firstSkip := ""
	for _, outcome := range outcomes {
		result := outcome.result
		if result.Skipped != "" {
			if firstSkip == "" {
				firstSkip = result.Skipped
			}
			continue
		}
		if !result.Incomplete {
			evaluated++
			merged.ImagesChecked++
		} else if firstSkip == "" {
			firstSkip = domain.SkipProviderError
		}
		merged.ImageCandidates = append(merged.ImageCandidates, result.ImageCandidates...)
		if merged.CheckedAt.IsZero() || (!result.CheckedAt.IsZero() && result.CheckedAt.Before(merged.CheckedAt)) {
			merged.CheckedAt = result.CheckedAt
		}
		for _, rule := range result.Flags {
			for _, match := range result.ImageMatches {
				if match.Rule != rule {
					continue
				}
				key := match.Rule + "\x00" + match.ListingImageURL + "\x00" + match.SourcePageURL + "\x00" + match.SourceImageURL
				if !seenEvidence[key] {
					seenEvidence[key] = true
					merged.ImageMatches = append(merged.ImageMatches, match)
				}
			}
			if matched[rule] {
				continue
			}
			matched[rule] = true
			group, known := weights[rule]
			if known {
				merged.Risk += group.Weight
				merged.Hard = merged.Hard || group.Hard
			}
			merged.Flags = append(merged.Flags, rule)
			detail := detailForRule(result, rule)
			if detail == "" {
				detail = "matched"
			}
			merged.Details = append(merged.Details, fmt.Sprintf("%s: image %d — %s", rule, outcome.index, detail))
		}
	}
	if evaluated == 0 && len(merged.Flags) == 0 {
		merged.Skipped = firstSkip
		if merged.Skipped == "" {
			merged.Skipped = domain.SkipProviderError
		}
	}
	merged.Risk = detect.Clamp(merged.Risk)
	return merged
}

func detailForRule(result domain.SignalResult, rule string) string {
	prefix := rule + ": "
	for _, detail := range result.Details {
		if strings.HasPrefix(detail, prefix) {
			return strings.TrimPrefix(detail, prefix)
		}
	}
	return ""
}

func (v *Vision) annotate(ctx context.Context, imageURL, feature string) (map[string]any, error) {
	if fresh, _ := ctx.Value(freshKey{}).(bool); !fresh {
		if data, _, found := v.store.GetEvidence(ctx, hashURL(imageURL), feature); found {
			trace.Log(ctx, "vision", "%s: fresh provider evidence cache hit", feature)
			return data, nil
		}
	}
	if cachedOnly, _ := ctx.Value(cachedOnlyKey{}).(bool); cachedOnly {
		return nil, cacheMissError{}
	}
	budgetFeature := FeatureWebDetection
	if feature == "TEXT_DETECTION" {
		budgetFeature = FeatureTextDetection
	}
	allowed, err := v.store.ReserveVisionUnit(ctx, budgetFeature, v.cap)
	if err != nil {
		return nil, err
	}
	if !allowed {
		v.meter.VisionUnitSkipped(budgetFeature)
		return nil, budgetError{}
	}
	v.meter.VisionUnitUsed(budgetFeature)
	trace.Log(ctx, "vision", "calling Google Vision %s", feature)

	body, err := json.Marshal(map[string]any{"requests": []any{map[string]any{
		"image":    map[string]any{"source": map[string]string{"imageUri": imageURL}},
		"features": []any{map[string]any{"type": feature, "maxResults": 10}},
	}}})
	if err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(v.endpoint)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if runtime.GOOS == "js" {
		req.Header.Set("js.fetch:redirect", "error")
	}
	if v.apiKey != "" {
		req.Header.Set("X-Goog-Api-Key", v.apiKey)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// The body may carry the key in an echoed request; report status only.
		return nil, fmt.Errorf("vision returned HTTP %d", resp.StatusCode)
	}
	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	response := firstResponse(decoded)
	if response == nil {
		return nil, fmt.Errorf("vision missing image response")
	}
	if field, present := response[featureField(feature)]; present {
		if _, ok := field.(map[string]any); !ok {
			return nil, fmt.Errorf("vision malformed annotation")
		}
	}
	if feature == "WEB_DETECTION" {
		// Validate nested result shapes before permissive map-based extraction.
		raw, _ := json.Marshal(response["webDetection"])
		var web struct {
			Pages []struct {
				URL  string `json:"url"`
				Full []struct {
					URL string `json:"url"`
				} `json:"fullMatchingImages"`
				Partial []struct {
					URL string `json:"url"`
				} `json:"partialMatchingImages"`
			} `json:"pagesWithMatchingImages"`
			Full []struct {
				URL string `json:"url"`
			} `json:"fullMatchingImages"`
			Partial []struct {
				URL string `json:"url"`
			} `json:"partialMatchingImages"`
			Similar []struct {
				URL string `json:"url"`
			} `json:"visuallySimilarImages"`
		}
		if json.Unmarshal(raw, &web) != nil {
			return nil, fmt.Errorf("vision malformed web results")
		}
	}
	if _, failed := response["error"]; failed {
		// Retain valid annotations, but never cache an error response.
		if _, usable := response[featureField(feature)].(map[string]any); !usable {
			return nil, fmt.Errorf("vision image annotation failed")
		}
		return decoded, nil
	}
	at := time.Now().UTC()
	decoded["checked_at"] = at.Format(time.RFC3339Nano)
	if err := v.store.PutEvidence(ctx, hashURL(imageURL), feature, decoded, at); err != nil {
		trace.Log(ctx, "vision", "provider evidence cache write failed")
	}
	return decoded, nil
}

func featureField(feature string) string {
	if feature == "WEB_DETECTION" {
		return "webDetection"
	}
	return "fullTextAnnotation"
}

type freshKey struct{}
type cachedOnlyKey struct{}

// WithCachedImages prevents provider calls when a content script lacks refresh authority.
func WithCachedImages(ctx context.Context) context.Context {
	return context.WithValue(ctx, cachedOnlyKey{}, true)
}

type cacheMissError struct{}

func (cacheMissError) Error() string { return "photo refresh required" }

func WithFreshImages(ctx context.Context) context.Context {
	return context.WithValue(ctx, freshKey{}, true)
}

type budgetError struct{}

func (budgetError) Error() string { return "vision budget exhausted" }
func skipReason(err error) string {
	if _, ok := err.(cacheMissError); ok {
		return domain.SkipPhotoRefreshRequired
	}
	if _, ok := err.(budgetError); ok {
		return domain.SkipBudgetExhausted
	}
	return domain.SkipProviderError
}
func responseTime(data map[string]any) time.Time {
	value, _ := data["checked_at"].(string)
	at, _ := time.Parse(time.RFC3339Nano, value)
	if at.IsZero() {
		return time.Now().UTC()
	}
	return at
}

// validateImageURL accepts only the documented Craigslist image host. Image
// URLs arrive from the page and are passed to Google for fetching, so checking
// a resolved IP locally cannot defend against a later DNS rebinding at the
// provider. A strict host allowlist does.
func validateImageURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("unparseable image URL")
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("unsupported image URL scheme %q", parsed.Scheme)
	}
	host := parsed.Hostname()
	if parsed.User != nil || !strings.EqualFold(host, craigslistImageHost) || parsed.Port() != "" {
		return fmt.Errorf("image URL host is not supported")
	}
	return nil
}

func hashURL(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func firstResponse(data map[string]any) map[string]any {
	responses, _ := data["responses"].([]any)
	if len(responses) == 0 {
		return nil
	}
	result, _ := responses[0].(map[string]any)
	return result
}
