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
	"os"
	"strings"
	"time"

	"github.com/aidandevaney/apartment-hunter/internal/database"
	"github.com/aidandevaney/apartment-hunter/internal/domain"
	"github.com/aidandevaney/apartment-hunter/internal/metrics"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	webFeature = "google_vision_web_detection"
	ocrFeature = "google_vision_text_detection"
)

// Vision uses the REST endpoint so it can be tested with a local httptest
// server. It supports either a restricted API key or application-default
// service-account credentials. Calls are disabled when neither is configured,
// preserving ingestion when an external provider is unavailable.
type Vision struct {
	store           *database.Store
	metrics         *metrics.Metrics
	endpoint        string
	apiKey          string
	enabledProvider bool
	cap             int
	client          *http.Client
}

func NewVision(store *database.Store, metric *metrics.Metrics, cap int) *Vision {
	endpoint := strings.TrimRight(os.Getenv("GOOGLE_VISION_ENDPOINT"), "/")
	if endpoint == "" {
		endpoint = "https://vision.googleapis.com/v1/images:annotate"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	apiKey := os.Getenv("GOOGLE_VISION_API_KEY")
	enabledProvider := apiKey != ""
	if apiKey == "" && os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" {
		if credentials, err := google.FindDefaultCredentials(context.Background(), "https://www.googleapis.com/auth/cloud-platform"); err == nil {
			client = oauth2.NewClient(context.Background(), credentials.TokenSource)
			client.Timeout = 3 * time.Second
			enabledProvider = true
		}
	}
	return &Vision{store: store, metrics: metric, endpoint: endpoint, apiKey: apiKey, enabledProvider: enabledProvider, cap: cap, client: client}
}
func (v *Vision) enabled() bool { return v.enabledProvider }
func (v *Vision) Enabled() bool { return v.enabledProvider }

type ReverseImage struct{ vision *Vision }

func NewReverseImage(v *Vision) *ReverseImage { return &ReverseImage{vision: v} }
func (*ReverseImage) Name() string            { return "reverse_image" }
func (s *ReverseImage) Evaluate(ctx context.Context, listing domain.Listing) (domain.SignalResult, error) {
	if len(listing.Images) == 0 || !s.vision.enabled() {
		return domain.SignalResult{Name: s.Name()}, nil
	}
	imageURL := listing.Images[0]
	hash := hashURL(imageURL)
	if cached, found, err := s.vision.store.GetCache(ctx, hash, s.Name()); err != nil || found {
		return cached, err
	}
	allowed, err := s.vision.store.ReserveVisionUnit(ctx, webFeature, s.vision.cap)
	if err != nil {
		return domain.SignalResult{}, err
	}
	if !allowed {
		s.vision.metrics.VisionSkipped.WithLabelValues(webFeature).Inc()
		return domain.SignalResult{Name: s.Name(), Details: []string{"skipped_monthly_budget"}}, nil
	}
	s.vision.metrics.VisionUnits.WithLabelValues(webFeature).Inc()
	data, err := s.vision.annotate(ctx, imageURL, "WEB_DETECTION")
	if err != nil {
		return domain.SignalResult{}, err
	}
	urls := webURLs(data)
	result := domain.SignalResult{Name: s.Name(), Details: urls}
	for _, match := range urls {
		lower := strings.ToLower(match)
		switch {
		case strings.Contains(lower, "zillow"), strings.Contains(lower, "redfin"), strings.Contains(lower, "realtor"), strings.Contains(lower, "mls"):
			result.Risk = .60
			result.Hard = true
			result.Flags = []string{"reverse_match_" + source(lower)}
		case strings.Contains(lower, "istock"), strings.Contains(lower, "shutterstock"), strings.Contains(lower, "unsplash"):
			if result.Risk < .35 {
				result.Risk = .35
				result.Flags = []string{"reverse_match_stock_photo"}
			}
		}
	}
	return result, s.vision.store.PutCache(ctx, hash, s.Name(), result)
}

type OCR struct{ vision *Vision }

func NewOCR(v *Vision) *OCR { return &OCR{vision: v} }
func (*OCR) Name() string   { return "ocr" }
func (s *OCR) Evaluate(ctx context.Context, listing domain.Listing) (domain.SignalResult, error) {
	if len(listing.Images) == 0 || !s.vision.enabled() {
		return domain.SignalResult{Name: s.Name()}, nil
	}
	imageURL := listing.Images[0]
	hash := hashURL(imageURL)
	if cached, found, err := s.vision.store.GetCache(ctx, hash, s.Name()); err != nil || found {
		return cached, err
	}
	allowed, err := s.vision.store.ReserveVisionUnit(ctx, ocrFeature, s.vision.cap)
	if err != nil {
		return domain.SignalResult{}, err
	}
	if !allowed {
		s.vision.metrics.VisionSkipped.WithLabelValues(ocrFeature).Inc()
		return domain.SignalResult{Name: s.Name(), Details: []string{"skipped_monthly_budget"}}, nil
	}
	s.vision.metrics.VisionUnits.WithLabelValues(ocrFeature).Inc()
	data, err := s.vision.annotate(ctx, imageURL, "TEXT_DETECTION")
	if err != nil {
		return domain.SignalResult{}, err
	}
	text := ocrText(data)
	lower := strings.ToLower(text)
	result := domain.SignalResult{Name: s.Name()}
	if strings.Contains(lower, "multiple listing service") || strings.Contains(lower, "mls") || strings.Contains(lower, "zillow") || strings.Contains(lower, "redfin") {
		result.Risk = .30
		result.Hard = true
		result.Flags = []string{"mls_watermark"}
		result.Details = []string{truncate(text, 200)}
	}
	return result, s.vision.store.PutCache(ctx, hash, s.Name(), result)
}

func (v *Vision) annotate(ctx context.Context, imageURL, feature string) (map[string]any, error) {
	body, _ := json.Marshal(map[string]any{"requests": []any{map[string]any{"image": map[string]any{"source": map[string]string{"imageUri": imageURL}}, "features": []any{map[string]any{"type": feature, "maxResults": 10}}}}})
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
	var decoded map[string]any
	if err = json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Vision returned HTTP %d", resp.StatusCode)
	}
	return decoded, nil
}
func hashURL(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func source(value string) string {
	switch {
	case strings.Contains(value, "zillow"):
		return "zillow"
	case strings.Contains(value, "redfin"):
		return "redfin"
	case strings.Contains(value, "realtor"):
		return "realtor"
	case strings.Contains(value, "mls"):
		return "mls"
	default:
		return "stock_photo"
	}
}
func response(data map[string]any) map[string]any {
	requests, _ := data["responses"].([]any)
	if len(requests) == 0 {
		return nil
	}
	result, _ := requests[0].(map[string]any)
	return result
}
func webURLs(data map[string]any) []string {
	r := response(data)
	if r == nil {
		return nil
	}
	detection, _ := r["webDetection"].(map[string]any)
	if detection == nil {
		return nil
	}
	var urls []string
	for _, key := range []string{"pagesWithMatchingImages", "fullMatchingImages", "partialMatchingImages"} {
		items, _ := detection[key].([]any)
		for _, item := range items {
			m, _ := item.(map[string]any)
			if value, _ := m["url"].(string); value != "" {
				urls = append(urls, value)
			}
		}
	}
	return urls
}
func ocrText(data map[string]any) string {
	r := response(data)
	if r == nil {
		return ""
	}
	full, _ := r["fullTextAnnotation"].(map[string]any)
	value, _ := full["text"].(string)
	return value
}
func truncate(v string, max int) string {
	if len(v) <= max {
		return v
	}
	return v[:max] + "…"
}
