package fraud

import (
	"context"
	"math"
	"testing"

	"github.com/aidandevaney/apartment-hunter/internal/domain"
)

func TestScorer_HardSignalsAreHighRiskAndNotUrgent(t *testing.T) {
	scorer := Scorer{QualifiedMax: .5, UrgentMax: .2, UrgentMinFit: .8, Preferences: Preferences{MaxPrice: 3000, MinBedrooms: 1, Locations: []string{"mission"}}}
	bedrooms := 1.0
	result := scorer.Score(domain.Listing{Price: 2500, Bedrooms: &bedrooms, Location: "Mission, San Francisco"}, []domain.SignalResult{{Name: "reverse_image", Risk: .60, Flags: []string{"reverse_match_zillow"}, Hard: true}, {Name: "ocr", Risk: .30, Flags: []string{"mls_watermark"}, Hard: true}})
	if result.Score < .8 || result.Qualified || result.Urgent {
		t.Fatalf("unexpected high-risk result: %#v", result)
	}
}

func TestFitScoreIsNeutralWithoutPreferences(t *testing.T) {
	if got := FitScore(domain.Listing{Price: 999999}, Preferences{}); got != .5 {
		t.Fatalf("got %v, want .5", got)
	}
}

type staticDetector struct {
	name   string
	result domain.SignalResult
}

func (d staticDetector) Name() string { return d.name }
func (d staticDetector) Evaluate(context.Context, domain.Listing) (domain.SignalResult, error) {
	return d.result, nil
}

func TestOrchestratorRunsAllSignals(t *testing.T) {
	o := NewOrchestrator([]Detector{staticDetector{"ocr", domain.SignalResult{Risk: .1}}, staticDetector{"ai_classifier", domain.SignalResult{Risk: .1}}, staticDetector{"reverse_image", domain.SignalResult{Risk: .1}}}, Scorer{QualifiedMax: .5, UrgentMax: .2, UrgentMinFit: .8})
	result, err := o.EvaluateListing(context.Background(), domain.Listing{})
	if err != nil || len(result.Signals) != 3 || math.Abs(result.Score-.3) > 1e-9 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
