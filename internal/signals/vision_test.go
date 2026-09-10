package signals

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/aidandevv/craig-extension/internal/cache"
	"github.com/aidandevv/craig-extension/internal/domain"
)

func newVision(t *testing.T, handler http.HandlerFunc, cap int) (*Vision, *cache.Store) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	return NewVision(store, NopMeter{}, Options{
		Endpoint:   server.URL,
		APIKey:     "test-key",
		MonthlyCap: cap,
	}), store
}

func webDetectionBody(urls ...string) string {
	pages := make([]any, 0, len(urls))
	for _, u := range urls {
		pages = append(pages, map[string]any{"url": u})
	}
	body, _ := json.Marshal(map[string]any{"responses": []any{
		map[string]any{"webDetection": map[string]any{"pagesWithMatchingImages": pages}},
	}})
	return string(body)
}

var realEstateGroups = []MatchGroup{
	{Rule: "reverse_image_real_estate", Tokens: []string{"zillow", "redfin", "realtor"}, Weight: 0.60, Hard: true},
	{Rule: "stock_photos", Tokens: []string{"shutterstock", "istockphoto"}, Weight: 0.35},
}

func listingWithImage() domain.Listing {
	return domain.Listing{Images: []string{"https://images.example.com/a.jpg"}}
}

func TestReverseImageFlagsRealEstateMatchAsHard(t *testing.T) {
	vision, _ := newVision(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(webDetectionBody("https://www.zillow.com/homedetails/123")))
	}, 100)

	got, err := NewReverseImage(vision, ReverseImageOptions{Groups: realEstateGroups}).
		Evaluate(context.Background(), listingWithImage())
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !got.Hard {
		t.Error("a real-estate match must set a hard flag")
	}
	if got.Risk != 0.60 {
		t.Errorf("got risk %v, want 0.60", got.Risk)
	}
	if len(got.Flags) != 1 || got.Flags[0] != "reverse_image_real_estate" {
		t.Errorf("got flags %v, want the owning rule name", got.Flags)
	}
	if len(got.ImageMatches) != 1 || got.ImageMatches[0].ListingImageURL != "https://images.example.com/a.jpg" || got.ImageMatches[0].SourcePageURL != "https://www.zillow.com/homedetails/123" {
		t.Errorf("image evidence = %+v", got.ImageMatches)
	}
}

func TestReverseImageCarriesMatchingImagePreviewWhenVisionSuppliesOne(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"responses": []any{
		map[string]any{"webDetection": map[string]any{
			"pagesWithMatchingImages": []any{map[string]any{"url": "https://www.zillow.com/homedetails/123"}},
			"fullMatchingImages":      []any{map[string]any{"url": "https://photos.zillowstatic.com/match.jpg"}},
		}},
	}})
	vision, _ := newVision(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}, 100)

	got, err := NewReverseImage(vision, ReverseImageOptions{Groups: realEstateGroups}).Evaluate(context.Background(), listingWithImage())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ImageMatches) != 1 || got.ImageMatches[0].SourceImageURL != "https://photos.zillowstatic.com/match.jpg" {
		t.Errorf("image evidence = %+v", got.ImageMatches)
	}
}

// One API call must serve every reverse-search rule; two matches sum.
func TestReverseImageSumsMultipleRuleHits(t *testing.T) {
	vision, _ := newVision(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(webDetectionBody(
			"https://www.redfin.com/x", "https://www.shutterstock.com/y")))
	}, 100)

	got, err := NewReverseImage(vision, ReverseImageOptions{Groups: realEstateGroups}).
		Evaluate(context.Background(), listingWithImage())
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(got.Flags) != 2 {
		t.Fatalf("got flags %v, want both rules attributed", got.Flags)
	}
	if got.Risk < 0.94 || got.Risk > 0.96 {
		t.Errorf("got risk %v, want 0.60 + 0.35", got.Risk)
	}
}

func TestReverseImageCleanPhotoScoresZero(t *testing.T) {
	vision, _ := newVision(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(webDetectionBody("https://someblog.example/post")))
	}, 100)

	got, err := NewReverseImage(vision, ReverseImageOptions{Groups: realEstateGroups}).
		Evaluate(context.Background(), listingWithImage())
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got.Risk != 0 || got.Hard || len(got.Flags) != 0 {
		t.Errorf("unmatched photo produced %+v, want a clean result", got)
	}
	if got.Skipped != "" {
		t.Errorf("clean result must not be marked skipped, got %q", got.Skipped)
	}
}

func TestReverseImageSecondCallIsServedFromCache(t *testing.T) {
	var calls int32
	vision, _ := newVision(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write([]byte(webDetectionBody("https://www.zillow.com/x")))
	}, 100)

	detector := NewReverseImage(vision, ReverseImageOptions{Groups: realEstateGroups})
	for i := 0; i < 2; i++ {
		if _, err := detector.Evaluate(context.Background(), listingWithImage()); err != nil {
			t.Fatalf("evaluate %d: %v", i, err)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("made %d provider calls for the same image, want 1", got)
	}
}

func TestReverseImageInspectsEveryListingImageAndOnlyScoresEachRuleOnce(t *testing.T) {
	var calls int32
	vision, _ := newVision(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		var request struct {
			Requests []struct {
				Image struct {
					Source struct {
						ImageURI string `json:"imageUri"`
					} `json:"source"`
				} `json:"image"`
			} `json:"requests"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Requests[0].Image.Source.ImageURI == "https://images.example.com/b.jpg" {
			_, _ = w.Write([]byte(webDetectionBody("https://www.zillow.com/homedetails/123")))
			return
		}
		_, _ = w.Write([]byte(webDetectionBody()))
	}, 100)

	got, err := NewReverseImage(vision, ReverseImageOptions{Groups: realEstateGroups}).Evaluate(context.Background(), domain.Listing{
		Images: []string{"https://images.example.com/a.jpg", "https://images.example.com/b.jpg"},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if calls := atomic.LoadInt32(&calls); calls != 2 {
		t.Errorf("Vision calls = %d, want one for each image", calls)
	}
	if got.Risk != 0.60 || !got.Hard || len(got.Flags) != 1 || got.Flags[0] != "reverse_image_real_estate" {
		t.Errorf("combined result = %+v", got)
	}
	if len(got.Details) != 1 || got.Details[0] != `reverse_image_real_estate: image 2 — matched "zillow"` {
		t.Errorf("details = %v", got.Details)
	}
	if len(got.ImageMatches) != 1 || got.ImageMatches[0].ListingImageURL != "https://images.example.com/b.jpg" {
		t.Errorf("image evidence = %+v", got.ImageMatches)
	}
}

func TestReverseImageReportsBudgetExhaustion(t *testing.T) {
	vision, store := newVision(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(webDetectionBody("https://www.zillow.com/x")))
	}, 1)
	if _, err := store.ReserveVisionUnit(context.Background(), FeatureWebDetection, 1); err != nil {
		t.Fatalf("pre-spend budget: %v", err)
	}

	got, err := NewReverseImage(vision, ReverseImageOptions{Groups: realEstateGroups}).
		Evaluate(context.Background(), domain.Listing{Images: []string{"https://images.example.com/other.jpg"}})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got.Skipped != domain.SkipBudgetExhausted {
		t.Errorf("got skipped %q, want %q", got.Skipped, domain.SkipBudgetExhausted)
	}
	if got.Risk != 0 {
		t.Errorf("a skipped signal must contribute no risk, got %v", got.Risk)
	}
}

func TestSignalsReportWhyTheyCouldNotRun(t *testing.T) {
	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cases := []struct {
		name    string
		vision  *Vision
		listing domain.Listing
		want    string
	}{
		{
			name:    "no images",
			vision:  NewVision(store, nil, Options{APIKey: "k", MonthlyCap: 10}),
			listing: domain.Listing{},
			want:    domain.SkipNoImages,
		},
		{
			name:    "no credentials",
			vision:  NewVision(store, nil, Options{MonthlyCap: 10}),
			listing: listingWithImage(),
			want:    domain.SkipNoAPIKey,
		},
		{
			name:    "non-public image host",
			vision:  NewVision(store, nil, Options{APIKey: "k", MonthlyCap: 10}),
			listing: domain.Listing{Images: []string{"http://127.0.0.1/secret.jpg"}},
			want:    domain.SkipMissingField,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewReverseImage(tc.vision, ReverseImageOptions{Groups: realEstateGroups}).
				Evaluate(context.Background(), tc.listing)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if got.Skipped != tc.want {
				t.Errorf("got skipped %q, want %q", got.Skipped, tc.want)
			}
		})
	}
}

func TestReverseImageSurvivesProviderError(t *testing.T) {
	vision, _ := newVision(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}, 100)

	got, err := NewReverseImage(vision, ReverseImageOptions{Groups: realEstateGroups}).
		Evaluate(context.Background(), listingWithImage())
	if err != nil {
		t.Fatalf("a provider failure must not fail the signal: %v", err)
	}
	if got.Skipped != domain.SkipProviderError {
		t.Errorf("got skipped %q, want %q", got.Skipped, domain.SkipProviderError)
	}
}

func TestOCRFlagsWatermark(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"responses": []any{
		map[string]any{"fullTextAnnotation": map[string]any{
			"text": "Provided by  Multiple Listing Service  #4471",
		}},
	}})
	vision, _ := newVision(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}, 100)

	got, err := NewOCR(vision, OCROptions{Groups: []MatchGroup{
		{Rule: "mls_watermark", Tokens: []string{"multiple listing service"}, Weight: 0.30, Hard: true},
	}}).Evaluate(context.Background(), listingWithImage())
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !got.Hard || got.Risk != 0.30 {
		t.Errorf("got %+v, want a hard 0.30 watermark flag", got)
	}
	if len(got.Flags) != 1 || got.Flags[0] != "mls_watermark" {
		t.Errorf("got flags %v, want [mls_watermark]", got.Flags)
	}
}

func TestOCRInspectsEveryListingImage(t *testing.T) {
	var calls int32
	vision, _ := newVision(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		var request struct {
			Requests []struct {
				Image struct {
					Source struct {
						ImageURI string `json:"imageUri"`
					} `json:"source"`
				} `json:"image"`
			} `json:"requests"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		text := "no watermark"
		if request.Requests[0].Image.Source.ImageURI == "https://images.example.com/b.jpg" {
			text = "Multiple Listing Service"
		}
		body, _ := json.Marshal(map[string]any{"responses": []any{
			map[string]any{"fullTextAnnotation": map[string]any{"text": text}},
		}})
		_, _ = w.Write(body)
	}, 100)

	got, err := NewOCR(vision, OCROptions{Groups: []MatchGroup{
		{Rule: "mls_watermark", Tokens: []string{"multiple listing service"}, Weight: 0.30, Hard: true},
	}}).Evaluate(context.Background(), domain.Listing{Images: []string{
		"https://images.example.com/a.jpg", "https://images.example.com/b.jpg",
	}})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if calls := atomic.LoadInt32(&calls); calls != 2 {
		t.Errorf("Vision calls = %d, want one for each image", calls)
	}
	if got.Risk != 0.30 || !got.Hard || len(got.Flags) != 1 || got.Flags[0] != "mls_watermark" {
		t.Errorf("combined result = %+v", got)
	}
}

func TestValidateImageURL(t *testing.T) {
	cases := map[string]bool{
		"https://images.example.com/a.jpg": true,
		"http://images.example.com/a.jpg":  true,
		"file:///etc/passwd":               false,
		"http://localhost/a.jpg":           false,
		"http://127.0.0.1/a.jpg":           false,
		"http://192.168.1.10/a.jpg":        false,
		"http://169.254.169.254/latest":    false,
		"https://":                         false,
	}
	for raw, wantOK := range cases {
		err := validateImageURL(raw)
		if wantOK && err != nil {
			t.Errorf("%s: unexpected rejection: %v", raw, err)
		}
		if !wantOK && err == nil {
			t.Errorf("%s: expected rejection, got none", raw)
		}
	}
}
