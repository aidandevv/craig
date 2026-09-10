// Package detect runs independent risk signals concurrently. It knows nothing
// about what the signals mean; scoring and presentation live elsewhere.
package detect

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/aidandevv/craig-extension/internal/domain"
)

// Detector is one independent check. Implementations must be safe to call
// concurrently and must not mutate the listing.
type Detector interface {
	Name() string
	Evaluate(context.Context, domain.Listing) (domain.SignalResult, error)
}

// Evaluate runs every detector concurrently and returns results sorted by name
// so output is deterministic. A detector that fails degrades to a skipped
// signal rather than failing the batch: one unavailable provider must never
// cost the user the rest of the analysis.
func Evaluate(ctx context.Context, detectors []Detector, listing domain.Listing) []domain.SignalResult {
	results := make(chan domain.SignalResult, len(detectors))
	var wg sync.WaitGroup
	for _, detector := range detectors {
		detector := detector
		wg.Add(1)
		go func() {
			defer wg.Done()
			started := time.Now()
			result, err := detector.Evaluate(ctx, listing)
			if err != nil {
				result = domain.SignalResult{
					Name:    detector.Name(),
					Skipped: domain.SkipProviderError,
					Details: []string{err.Error()},
				}
			}
			if result.Name == "" {
				result.Name = detector.Name()
			}
			result.Risk = Clamp(result.Risk)
			result.LatencyMS = time.Since(started).Milliseconds()
			results <- result
		}()
	}
	wg.Wait()
	close(results)

	all := make([]domain.SignalResult, 0, len(detectors))
	for result := range results {
		all = append(all, result)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	return all
}

// Clamp bounds a risk contribution to [0,1].
func Clamp(v float64) float64 { return math.Max(0, math.Min(1, v)) }
