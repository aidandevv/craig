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
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/aidandevv/craig-extension/internal/cache"
	"github.com/aidandevv/craig-extension/internal/detect"
	"github.com/aidandevv/craig-extension/internal/domain"
)

// Vision bills each feature separately, so each carries its own budget line.
const (
	FeatureWebDetection  = "web_detection"
	FeatureTextDetection = "text_detection"
)

const defaultEndpoint = "https://vision.googleapis.com/v1/images:annotate"

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
	store    *cache.Store
	meter    Meter
	endpoint string
	apiKey   string
	enabled  bool
	cap      int
	client   *http.Client
}

// NewVision builds a client. It never fails on missing credentials: an
// unconfigured Vision is a disabled Vision, which is a supported mode.
func NewVision(store *cache.Store, meter Meter, opts Options) *Vision {
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
		client:   &http.Client{Timeout: timeout},
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

// call performs the shared cache-check, budget-reserve, request cycle. It
// returns a skipped result when the signal cannot run, which the caller passes
// through unchanged rather than treating as a clean pass.
func (v *Vision) call(ctx context.Context, name, feature string, listing domain.Listing) (domain.SignalResult, string, bool) {
	if len(listing.Images) == 0 {
		return domain.SignalResult{Name: name, Skipped: domain.SkipNoImages}, "", false
	}
	if !v.enabled {
		return domain.SignalResult{Name: name, Skipped: domain.SkipNoAPIKey}, "", false
	}
	imageURL := listing.Images[0]
	if err := validateImageURL(imageURL); err != nil {
		return domain.SignalResult{
			Name:    name,
			Skipped: domain.SkipMissingField,
			Details: []string{err.Error()},
		}, "", false
	}

	hash := hashURL(imageURL)
	if cached, found, err := v.store.Get(ctx, hash, name); err == nil && found {
		return cached, hash, false
	}

	allowed, err := v.store.ReserveVisionUnit(ctx, feature, v.cap)
	if err != nil {
		return domain.SignalResult{
			Name:    name,
			Skipped: domain.SkipProviderError,
			Details: []string{err.Error()},
		}, hash, false
	}
	if !allowed {
		v.meter.VisionUnitSkipped(feature)
		return domain.SignalResult{Name: name, Skipped: domain.SkipBudgetExhausted}, hash, false
	}
	v.meter.VisionUnitUsed(feature)
	return domain.SignalResult{Name: name}, hash, true
}

func (v *Vision) annotate(ctx context.Context, imageURL, feature string) (map[string]any, error) {
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
	if v.apiKey != "" {
		q := endpoint.Query()
		q.Set("key", v.apiKey)
		endpoint.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

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
	return decoded, nil
}

// validateImageURL rejects anything that is not a public http(s) resource.
// Image URLs arrive from whatever page the user is viewing, so they are
// untrusted input even though Google, not this process, performs the fetch.
func validateImageURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("unparseable image URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported image URL scheme %q", parsed.Scheme)
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("image URL has no host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return fmt.Errorf("image URL points at a non-public address")
		}
	}
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("image URL points at a non-public address")
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
