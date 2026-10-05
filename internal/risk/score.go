// Package risk turns signal results into a score, a band, and an explained
// assessment. It is the only package that decides what a risk number means.
package risk

import (
	"github.com/aidandevv/craig/internal/detect"
	"github.com/aidandevv/craig/internal/domain"
	"github.com/aidandevv/craig/internal/rules"
)

const (
	BandLow      = "low"
	BandCaution  = "caution"
	BandElevated = "elevated"
	BandHigh     = "high"
)

// Score sums signals that actually ran, clamps the arithmetic to [0,1], and
// assigns the user-facing band. A hard finding is categorical and therefore
// pins its band to high without altering the displayed arithmetic score.
func Score(results []domain.SignalResult, bands rules.RiskBands) (score float64, band string, hard bool) {
	for _, result := range results {
		if result.Skipped != "" {
			continue
		}
		score += result.Risk
		if result.Hard {
			hard = true
		}
	}
	score = detect.Clamp(score)
	switch {
	case hard || score >= bands.High:
		return score, BandHigh, hard
	case score >= bands.Elevated:
		return score, BandElevated, hard
	case score >= bands.Caution:
		return score, BandCaution, hard
	default:
		return score, BandLow, hard
	}
}
