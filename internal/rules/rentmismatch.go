package rules

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/aidandevv/craig/internal/detect"
	"github.com/aidandevv/craig/internal/domain"
)

// dollarAmount is deliberately title-only in this check. Rental descriptions
// commonly mention deposits, fees, and concessions, while a title is where a
// marketplace presents a competing advertised rent.
var dollarAmount = regexp.MustCompile(`\$\s*([0-9][0-9,]*(?:\.[0-9]{1,2})?)`)

// rentPriceMismatch compares the structured price scraped from the listing
// header with rent amounts prominently advertised in its title. It surfaces a
// bait-and-switch or stale-price discrepancy as a caution, never fraud proof.
type rentPriceMismatch struct {
	name     string
	weight   float64
	minRatio float64
}

func newRentPriceMismatch(name string, r Rule, weight float64) (detect.Detector, error) {
	return &rentPriceMismatch{name: name, weight: weight, minRatio: r.RentPriceMismatchRatio}, nil
}

func (r *rentPriceMismatch) Name() string { return r.name }

func (r *rentPriceMismatch) Evaluate(_ context.Context, listing domain.Listing) (domain.SignalResult, error) {
	if listing.Price <= 0 || strings.TrimSpace(listing.Title) == "" {
		return domain.SignalResult{Name: r.name, Skipped: domain.SkipRentPriceInputsMissing,
			Details: []string{r.name + ": requires a listed page price and a title with a dollar amount"}}, nil
	}

	for _, match := range dollarAmount.FindAllStringSubmatch(listing.Title, -1) {
		advertised, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
		if err != nil || advertised <= 0 || int(math.Round(advertised)) == listing.Price {
			continue
		}
		ratio := math.Abs(advertised-float64(listing.Price)) / float64(listing.Price)
		if ratio < r.minRatio {
			continue
		}
		percent := int(math.Round(ratio * 100))
		detail := fmt.Sprintf("page price is %s, but the title also advertises %s (%d%% difference). Confirm the current rent before applying or paying a fee; this is not proof of fraud.",
			formatDollars(float64(listing.Price)), formatDollars(advertised), percent)
		return domain.SignalResult{Name: r.name, Risk: detect.Clamp(r.weight), Flags: []string{r.name}, Details: []string{r.name + ": " + detail}}, nil
	}
	return domain.SignalResult{Name: r.name}, nil
}
