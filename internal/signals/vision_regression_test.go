package signals

import (
	"context"
	"github.com/aidandevv/craig-extension/internal/domain"
	"net/http"
	"testing"
)

func TestAnnotationFailuresAreRetried(t *testing.T) {
	for _, body := range []string{`{"responses":[{"error":{"code":14,"message":"fetch failed"}}]}`, `{}`, `{"responses":[]}`, `{"responses":[null]}`, `{"responses":[{"webDetection":"bad"}]}`, `{"responses":[{"webDetection":{"pagesWithMatchingImages":"bad"}}]}`} {
		t.Run(body, func(t *testing.T) {
			calls := 0
			v, _ := newVision(t, func(w http.ResponseWriter, r *http.Request) { calls++; w.Write([]byte(body)) }, 100)
			detector := NewReverseImage(v, ReverseImageOptions{Groups: realEstateGroups})
			for i := 0; i < 2; i++ {
				got, err := detector.Evaluate(context.Background(), listingWithImage())
				if err != nil || got.Skipped != domain.SkipProviderError {
					t.Fatalf("result=%+v err=%v", got, err)
				}
			}
			if calls != 2 {
				t.Fatalf("failure cached: calls=%d", calls)
			}
		})
	}
}
func TestEvidenceRescoredAndFreshCheckRespectsBudget(t *testing.T) {
	calls := 0
	v, _ := newVision(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(webDetectionBody("https://www.zillow.com/home")))
	}, 2)
	ctx := context.Background()
	first := NewReverseImage(v, ReverseImageOptions{Groups: []MatchGroup{{Rule: "reverse_image_real_estate", Tokens: []string{"redfin"}, Weight: .6}}})
	first.Evaluate(ctx, listingWithImage())
	detector := NewReverseImage(v, ReverseImageOptions{Groups: realEstateGroups})
	got, _ := detector.Evaluate(ctx, listingWithImage())
	if !got.Hard || calls != 1 {
		t.Fatalf("evidence not rescored: %+v calls=%d", got, calls)
	}
	got, _ = detector.Evaluate(WithFreshImages(ctx), listingWithImage())
	if !got.Hard || calls != 2 {
		t.Fatalf("fresh check used cache: %+v calls=%d", got, calls)
	}
	got, _ = detector.Evaluate(WithFreshImages(ctx), listingWithImage())
	if got.Skipped != domain.SkipBudgetExhausted || calls != 2 {
		t.Fatalf("budget bypassed: %+v calls=%d", got, calls)
	}
}
func TestPartialCoverageKeepsPositiveEvidence(t *testing.T) {
	got := mergeImageOutcomes("reverse_image", realEstateGroups, []imageOutcome{
		{index: 1, result: domain.SignalResult{Name: "reverse_image", Flags: []string{"reverse_image_real_estate"}}},
		{index: 2, result: domain.SignalResult{Skipped: domain.SkipProviderError}},
	})
	if !got.Hard || got.ImagesChecked != 1 || got.ImagesTotal != 2 || got.Skipped != "" {
		t.Fatalf("result=%+v", got)
	}
}
func TestSimilarCandidatesAreIgnored(t *testing.T) {
	v, _ := newVision(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"responses":[{"webDetection":{"visuallySimilarImages":[{"url":"https://photos.zillowstatic.com/room.jpg"}]}}]}`))
	}, 100)
	got, _ := NewReverseImage(v, ReverseImageOptions{Groups: realEstateGroups}).Evaluate(context.Background(), listingWithImage())
	if got.Hard || len(got.Flags) > 0 || len(got.ImageCandidates) != 0 {
		t.Fatalf("result=%+v", got)
	}
}
func TestPageEvidenceDoesNotUseUnrelatedGlobalImage(t *testing.T) {
	v, _ := newVision(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"responses":[{"webDetection":{"pagesWithMatchingImages":[{"url":"https://www.zillow.com/home","partialMatchingImages":[{"url":"https://cdn.example/correct.jpg"}]}],"fullMatchingImages":[{"url":"https://other.example/wrong.jpg"}]}}]}`))
	}, 100)
	got, _ := NewReverseImage(v, ReverseImageOptions{Groups: realEstateGroups}).Evaluate(context.Background(), listingWithImage())
	if len(got.ImageMatches) != 1 || got.ImageMatches[0].SourceImageURL != "https://cdn.example/correct.jpg" {
		t.Fatalf("result=%+v", got)
	}
}
func TestEmptyAnnotationIsSuccessfulAndLegacyNegativeIgnored(t *testing.T) {
	calls := 0
	v, store := newVision(t, func(w http.ResponseWriter, r *http.Request) { calls++; w.Write([]byte(`{"responses":[{}]}`)) }, 100)
	store.Put(context.Background(), hashURL(listingWithImage().Images[0]), "reverse_image", domain.SignalResult{Name: "reverse_image"})
	got, _ := NewReverseImage(v, ReverseImageOptions{Groups: realEstateGroups}).Evaluate(context.Background(), listingWithImage())
	if got.Skipped != "" || got.ImagesChecked != 1 || calls != 1 {
		t.Fatalf("result=%+v calls=%d", got, calls)
	}
}

func TestErrorWithUsableAnnotationKeepsMatchButDoesNotCache(t *testing.T) {
	calls := 0
	v, _ := newVision(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"responses":[{"error":{"code":14},"webDetection":{"pagesWithMatchingImages":[{"url":"https://www.zillow.com/home"}]}}]}`))
	}, 100)
	detector := NewReverseImage(v, ReverseImageOptions{Groups: realEstateGroups})
	for i := 0; i < 2; i++ {
		got, _ := detector.Evaluate(context.Background(), listingWithImage())
		if !got.Hard || got.ImagesChecked != 0 || got.ImagesTotal != 1 {
			t.Fatalf("result=%+v", got)
		}
	}
	if calls != 2 {
		t.Fatal("partially failed annotation cached")
	}
}
