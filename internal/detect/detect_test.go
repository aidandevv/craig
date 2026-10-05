package detect

import (
	"context"
	"errors"
	"testing"

	"github.com/aidandevv/craig/internal/domain"
)

type stub struct {
	name   string
	result domain.SignalResult
	err    error
}

func (s stub) Name() string { return s.name }
func (s stub) Evaluate(context.Context, domain.Listing) (domain.SignalResult, error) {
	return s.result, s.err
}

func TestEvaluateSortsByName(t *testing.T) {
	got := Evaluate(context.Background(), []Detector{
		stub{name: "zeta"}, stub{name: "alpha"}, stub{name: "mid"},
	}, domain.Listing{})

	want := []string{"alpha", "mid", "zeta"}
	if len(got) != len(want) {
		t.Fatalf("got %d results, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("position %d: got %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestEvaluateDegradesFailingDetector(t *testing.T) {
	got := Evaluate(context.Background(), []Detector{
		stub{name: "broken", err: errors.New("provider down")},
		stub{name: "healthy", result: domain.SignalResult{Risk: 0.4}},
	}, domain.Listing{})

	if len(got) != 2 {
		t.Fatalf("got %d results, want 2", len(got))
	}
	if got[0].Name != "broken" || got[0].Skipped != domain.SkipProviderError {
		t.Errorf("failing detector: got name=%q skipped=%q, want broken/%s",
			got[0].Name, got[0].Skipped, domain.SkipProviderError)
	}
	if got[0].Risk != 0 {
		t.Errorf("a failed signal must contribute no risk, got %v", got[0].Risk)
	}
	if got[1].Risk != 0.4 {
		t.Errorf("healthy detector: got risk %v, want 0.4", got[1].Risk)
	}
}

func TestEvaluateClampsAndNamesResults(t *testing.T) {
	got := Evaluate(context.Background(), []Detector{
		stub{name: "over", result: domain.SignalResult{Risk: 3.2}},
		stub{name: "under", result: domain.SignalResult{Risk: -1}},
	}, domain.Listing{})

	if got[0].Risk != 1 {
		t.Errorf("risk above 1 must clamp to 1, got %v", got[0].Risk)
	}
	if got[1].Risk != 0 {
		t.Errorf("negative risk must clamp to 0, got %v", got[1].Risk)
	}
	for _, r := range got {
		if r.Name == "" {
			t.Error("result left without a name")
		}
	}
}

func TestEvaluateNoDetectors(t *testing.T) {
	if got := Evaluate(context.Background(), nil, domain.Listing{}); len(got) != 0 {
		t.Errorf("got %d results, want 0", len(got))
	}
}

type panickingDetector struct{}

func (panickingDetector) Name() string { return "panic" }
func (panickingDetector) Evaluate(context.Context, domain.Listing) (domain.SignalResult, error) {
	panic("sensitive host error")
}

func TestEvaluateRecoversDetectorPanic(t *testing.T) {
	got := Evaluate(context.Background(), []Detector{panickingDetector{}, stub{name: "healthy"}}, domain.Listing{})
	if len(got) != 2 || got[1].Skipped != domain.SkipProviderError || got[1].Details[0] != "detector failed" {
		t.Fatalf("panic was not safely isolated: %+v", got)
	}
}
