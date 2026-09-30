// Package detect runs independent risk signals concurrently. It knows nothing
// about what the signals mean; scoring and presentation live elsewhere.
package detect

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/aidandevv/craig-extension/internal/domain"
	"github.com/aidandevv/craig-extension/internal/trace"
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
			// Each detector owns its goroutine; the caller cannot recover its panic.
			defer func() {
				if recover() != nil {
					results <- domain.SignalResult{Name: detector.Name(), Skipped: domain.SkipProviderError, Details: []string{"detector failed"}}
				}
			}()
			started := time.Now()
			trace.Log(ctx, "detector", "%s started", detector.Name())
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
			trace.Log(ctx, "detector", "%s finished in %dms (%s)", detector.Name(), result.LatencyMS, outcome(result))
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

func outcome(result domain.SignalResult) string {
	if result.Skipped != "" {
		return "skipped: " + result.Skipped
	}
	if len(result.Flags) > 0 {
		return fmt.Sprintf("matched %d rule(s)", len(result.Flags))
	}
	return "no match"
}
