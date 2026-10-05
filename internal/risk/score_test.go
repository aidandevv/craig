package risk

import (
	"testing"

	"github.com/aidandevv/craig/internal/domain"
	"github.com/aidandevv/craig/internal/rules"
)

var testBands = rules.RiskBands{Caution: .30, Elevated: .55, High: .75}

func TestScoreSumsContributions(t *testing.T) {
	score, band, hard := Score([]domain.SignalResult{{Name: "a", Risk: .45}, {Name: "b", Risk: .28}}, testBands)
	if score != .73 || band != BandElevated || hard {
		t.Errorf("got (%v, %q, %v)", score, band, hard)
	}
}

func TestScoreClampsAtBounds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		risks []float64
		want  float64
		band  string
	}{
		{"above one", []float64{.60, .45, .35}, 1, BandHigh},
		{"below zero", []float64{-.25}, 0, BandLow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			results := make([]domain.SignalResult, len(tc.risks))
			for i, v := range tc.risks {
				results[i] = domain.SignalResult{Risk: v}
			}
			got, band, _ := Score(results, testBands)
			if got != tc.want || band != tc.band {
				t.Errorf("got (%v, %q), want (%v, %q)", got, band, tc.want, tc.band)
			}
		})
	}
}

func TestScoreBandBoundaries(t *testing.T) {
	for _, tc := range []struct {
		score float64
		want  string
	}{{0, BandLow}, {.29, BandLow}, {.30, BandCaution}, {.54, BandCaution}, {.55, BandElevated}, {.74, BandElevated}, {.75, BandHigh}, {1, BandHigh}} {
		_, got, _ := Score([]domain.SignalResult{{Risk: tc.score}}, testBands)
		if got != tc.want {
			t.Errorf("score %v: got %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestHardFlagPinsBandWithoutChangingScore(t *testing.T) {
	score, band, hard := Score([]domain.SignalResult{{Name: "watermark", Risk: .30, Hard: true}}, testBands)
	if score != .30 || band != BandHigh || !hard {
		t.Errorf("got (%v, %q, %v)", score, band, hard)
	}
}

func TestGreenAndSkippedSignalsDoNotChangeScore(t *testing.T) {
	base, _, _ := Score([]domain.SignalResult{{Name: "payment", Risk: .45}}, testBands)
	got, band, hard := Score([]domain.SignalResult{
		{Name: "payment", Risk: .45},
		{Name: "direct_phone", Risk: 0},
		{Name: "reverse_image", Risk: .99, Hard: true, Skipped: domain.SkipNoAPIKey},
	}, testBands)
	if got != base || band != BandCaution || hard {
		t.Errorf("got (%v, %q, %v), base %v", got, band, hard, base)
	}
}

func TestScoreWithNoSignals(t *testing.T) {
	score, band, hard := Score(nil, testBands)
	if score != 0 || band != BandLow || hard {
		t.Errorf("got (%v, %q, %v)", score, band, hard)
	}
}
