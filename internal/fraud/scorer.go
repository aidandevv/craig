// Package fraud evaluates independent risk signals concurrently and produces an
// explainable, bounded score. Signals are indicators, never a claim of fraud.
package fraud

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aidandevaney/apartment-hunter/internal/domain"
)

type Detector interface {
	Name() string
	Evaluate(context.Context, domain.Listing) (domain.SignalResult, error)
}

type Preferences struct {
	MinPrice    int
	MaxPrice    int
	MinBedrooms float64
	Locations   []string
}

type Scorer struct {
	QualifiedMax float64
	UrgentMax    float64
	UrgentMinFit float64
	Preferences  Preferences
}

func (s Scorer) Score(listing domain.Listing, signals []domain.SignalResult) domain.FraudResult {
	flags, hardFlags := make([]string, 0), make([]string, 0)
	// Signal risks are already calibrated contribution values, so sum is simple,
	// explainable, and bounded. A hard flag is never silently discarded.
	score := 0.0
	for _, signal := range signals {
		score += clamp(signal.Risk)
		flags = append(flags, signal.Flags...)
		if signal.Hard {
			hardFlags = append(hardFlags, signal.Flags...)
		}
	}
	fit := FitScore(listing, s.Preferences)
	sort.Strings(flags)
	flags = compact(flags)
	sort.Strings(hardFlags)
	hardFlags = compact(hardFlags)
	score = clamp(score)
	return domain.FraudResult{
		Score: score, Flags: flags, HardFlags: hardFlags, Signals: signals, FitScore: fit,
		Qualified: score <= s.QualifiedMax,
		Urgent:    score <= s.UrgentMax && fit >= s.UrgentMinFit && len(hardFlags) == 0,
	}
}

// FitScore is deliberately deterministic. It rewards price, bedroom, and
// location preference matches; it does not make a model-based value judgment.
func FitScore(l domain.Listing, p Preferences) float64 {
	parts, total := 0.0, 0.0
	if p.MinPrice > 0 || p.MaxPrice > 0 {
		total += .45
		if (p.MinPrice == 0 || l.Price >= p.MinPrice) && (p.MaxPrice == 0 || l.Price <= p.MaxPrice) {
			parts += .45
		} else if p.MaxPrice > 0 && l.Price > 0 && l.Price <= int(float64(p.MaxPrice)*1.10) {
			parts += .20
		}
	}
	if p.MinBedrooms > 0 {
		total += .25
		if l.Bedrooms != nil && *l.Bedrooms >= p.MinBedrooms {
			parts += .25
		}
	}
	if len(p.Locations) > 0 {
		total += .30
		location := strings.ToLower(l.Location)
		for _, preferred := range p.Locations {
			if strings.Contains(location, strings.ToLower(preferred)) {
				parts += .30
				break
			}
		}
	}
	if total == 0 {
		return .5
	} // neutral until the user supplies preferences
	return math.Round((parts/total)*100) / 100
}

type Orchestrator struct {
	detectors []Detector
	scorer    Scorer
}

func NewOrchestrator(detectors []Detector, scorer Scorer) *Orchestrator {
	return &Orchestrator{detectors: detectors, scorer: scorer}
}

func (o *Orchestrator) EvaluateListing(ctx context.Context, listing domain.Listing) (domain.FraudResult, error) {
	started := time.Now()
	results := make(chan domain.SignalResult, len(o.detectors))
	var wg sync.WaitGroup
	for _, detector := range o.detectors {
		detector := detector
		wg.Add(1)
		go func() {
			defer wg.Done()
			signalStarted := time.Now()
			result, err := detector.Evaluate(ctx, listing)
			if err != nil {
				result = domain.SignalResult{Name: detector.Name(), Details: []string{"signal_error:" + err.Error()}}
			}
			if result.Name == "" {
				result.Name = detector.Name()
			}
			result.LatencyMS = time.Since(signalStarted).Milliseconds()
			results <- result
		}()
	}
	wg.Wait()
	close(results)
	all := make([]domain.SignalResult, 0, len(o.detectors))
	for result := range results {
		all = append(all, result)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	result := o.scorer.Score(listing, all)
	result.ElapsedMS = time.Since(started).Milliseconds()
	return result, nil
}

func clamp(v float64) float64 { return math.Max(0, math.Min(1, v)) }
func compact(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	result := items[:0]
	for _, item := range items {
		if item != "" && (len(result) == 0 || result[len(result)-1] != item) {
			result = append(result, item)
		}
	}
	return result
}
