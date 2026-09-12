package rules

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/aidandevv/craig-extension/internal/detect"
	"github.com/aidandevv/craig-extension/internal/domain"
	"github.com/aidandevv/craig-extension/internal/marketdata"
)

// marketRentCheck identifies a listed monthly USD rent that is unusually low
// against a bundled HUD SAFMR benchmark. It deliberately cannot produce a
// hard finding: the comparison is useful evidence to verify a listing, not
// evidence that the listing is fraudulent.
type marketRentCheck struct {
	name     string
	weight   float64
	lowRatio float64
}

func newMarketRentCheck(name string, r Rule, weight float64) (detect.Detector, error) {
	return &marketRentCheck{name: name, weight: weight, lowRatio: r.MarketRentLowRatio}, nil
}

func (m *marketRentCheck) Name() string { return m.name }

func (m *marketRentCheck) Evaluate(_ context.Context, listing domain.Listing) (domain.SignalResult, error) {
	if listing.Price <= 0 || listing.Currency != "USD" || listing.RentPeriod != "monthly" || listing.Bedrooms == nil {
		return m.unavailable(domain.SkipMarketRentInputsMissing, "requires a listed monthly USD rent and bedroom count")
	}
	benchmark, found := marketdata.Lookup(listing.ZIPCode, *listing.Bedrooms)
	if !found {
		return m.unavailable(domain.SkipMarketRentBenchmarkUnavailable, "requires a supported five-digit ZIP code and a studio-to-four-bedroom HUD benchmark")
	}

	ratio := float64(listing.Price) / float64(benchmark.GrossRent)
	if ratio > m.lowRatio {
		return domain.SignalResult{Name: m.name}, nil
	}
	belowPercent := int(math.Round((1 - ratio) * 100))
	detail := fmt.Sprintf(
		"%s/month is %d%% below HUD FY %d %s/month %s 40th-percentile gross-rent benchmark for ZIP %s (%s). The benchmark includes estimated tenant-paid utilities and is not proof of fraud.",
		formatDollars(float64(listing.Price)), belowPercent, benchmark.Source.FiscalYear,
		formatDollars(float64(benchmark.GrossRent)), bedroomLabel(benchmark.Bedrooms), benchmark.ZIPCode,
		strings.Join(benchmark.Areas, " / "),
	)
	return domain.SignalResult{
		Name:    m.name,
		Risk:    detect.Clamp(m.weight),
		Flags:   []string{m.name},
		Details: []string{m.name + ": " + detail},
	}, nil
}

func (m *marketRentCheck) unavailable(skip, reason string) (domain.SignalResult, error) {
	return domain.SignalResult{
		Name:    m.name,
		Skipped: skip,
		Details: []string{m.name + ": " + reason},
	}, nil
}

func bedroomLabel(bedrooms int) string {
	if bedrooms == 0 {
		return "studio"
	}
	return fmt.Sprintf("%d-bedroom", bedrooms)
}
