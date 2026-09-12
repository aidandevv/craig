package rules

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/aidandevv/craig-extension/internal/detect"
	"github.com/aidandevv/craig-extension/internal/domain"
)

// A fee amount more than this multiple of its normal ceiling is treated as
// categorically out of the ordinary rather than merely worth a caution.
const severeFeeMultiplier = 3.0

// Fee-check kinds let defaults explain a nonstandard fee separately from a
// high amount for an otherwise familiar fee. An omitted kind remains combined
// for compatibility with existing hand-authored rules files.
const (
	FeeCheckKindNonstandard  = "nonstandard"
	FeeCheckKindHighStandard = "high_standard"
	FeeCheckKindCombined     = "combined"
)

var (
	// These phrases are deliberately narrow. A holding fee can be legitimate,
	// but it commits a renter to money before the normal lease process is
	// complete, so it is useful as a caution-level signal regardless of amount.
	reHoldingFee = regexp.MustCompile(`(?i)\b(?:holding|hold|reservation|reserve)\s+(?:fee|deposit|charge)\b|\b(?:fee|deposit|charge)\s+(?:to|for)\s+(?:hold|reserve)\b`)

	// Listings put the amount either after "application fee" or before it.
	// Both expressions intentionally require a dollar sign or a numeric amount
	// immediately adjacent to the fee phrase, avoiding unrelated prices.
	reApplicationFeeAfter  = regexp.MustCompile(`(?i)\b(?:application|app)(?:\s+(?:processing|screening|administrative))?\s+(?:fee|charge|cost)\s*(?:(?:is|of|costs|due)\s*)?[:=-]?\s*\$?\s*([0-9][0-9,]*(?:\.[0-9]{1,2})?)`)
	reApplicationFeeBefore = regexp.MustCompile(`(?i)\$\s*([0-9][0-9,]*(?:\.[0-9]{1,2})?)\s*(?:application|app)(?:\s+(?:processing|screening|administrative))?\s+(?:fee|charge|cost)\b`)
)

type nonstandardFeePattern struct {
	label string
	re    *regexp.Regexp
}

// nonstandardFeePatterns names up-front charges that are distinct from a
// routine application/screening or move-in charge. They are not declared
// unlawful here—rental-fee rules vary by jurisdiction—but they are useful
// prompts to ask about purpose, refundability, and timing before paying.
var nonstandardFeePatterns = []nonstandardFeePattern{
	{"hold or reservation fee", reHoldingFee},
	{"key money", regexp.MustCompile(`(?i)\bkey\s+money\b`)},
	{"option fee", regexp.MustCompile(`(?i)\boption\s+fee\b`)},
	{"commitment fee", regexp.MustCompile(`(?i)\bcommitment\s+fee\b`)},
	{"priority fee", regexp.MustCompile(`(?i)\bpriority\s+fee\b`)},
	{"waitlist fee", regexp.MustCompile(`(?i)\bwait(?:ing)?[\s-]*list\s+fee\b`)},
	{"application deposit", regexp.MustCompile(`(?i)\bapplication\s+deposit\b`)},
	{"good-faith deposit", regexp.MustCompile(`(?i)\bgood[\s-]*faith\s+deposit\b`)},
	{"pre-lease fee", regexp.MustCompile(`(?i)\bpre[\s-]*lease\s+(?:fee|deposit)\b`)},
}

// feeCategory recognizes one named, one-time rental fee and the dollar
// amount above which it is worth mentioning at all ("normal"). Amounts more
// than severeFeeMultiplier times normal are flagged as categorically out of
// the ordinary rather than a routine caution.
type feeCategory struct {
	label  string
	after  *regexp.Regexp
	before *regexp.Regexp
	normal float64
}

// extraFeeCategories covers the common named fees beyond the application fee
// (which keeps its own YAML-configurable threshold). Normal ceilings are
// rough, deliberately generous market norms: the goal is to catch amounts far
// outside them, not to police ordinary pricing.
var extraFeeCategories = buildExtraFeeCategories()

func buildExtraFeeCategories() []feeCategory {
	specs := []struct {
		label   string
		pattern string
		normal  float64
	}{
		{"credit or background check fee", `(?:credit|background)[\s-]*check`, 50},
		{"tenant screening fee", `(?:tenant\s+)?screening`, 50},
		{"processing fee", `processing`, 75},
		{"administrative fee", `admin(?:istrative)?`, 150},
		{"lease initiation fee", `lease[\s-]*(?:initiation|setup)`, 300},
		{"move-in fee", `move[\s-]?in`, 300},
		{"amenity fee", `amenit(?:y|ies)`, 150},
		{"convenience fee", `convenience`, 50},
		{"pet fee", `pet`, 300},
	}
	categories := make([]feeCategory, 0, len(specs))
	for _, spec := range specs {
		after, before := feeAmountRegexes(spec.pattern)
		categories = append(categories, feeCategory{label: spec.label, after: after, before: before, normal: spec.normal})
	}
	return categories
}

// feeAmountRegexes builds the same "name then amount" / "amount then name"
// pair used for the application fee, parameterized on the fee's name
// pattern so every named fee is recognized the same way.
func feeAmountRegexes(namePattern string) (after, before *regexp.Regexp) {
	after = regexp.MustCompile(`(?i)\b(?:` + namePattern + `)\s+(?:fee|charge|cost)\s*(?:(?:is|of|costs|due)\s*)?[:=-]?\s*\$?\s*([0-9][0-9,]*(?:\.[0-9]{1,2})?)`)
	before = regexp.MustCompile(`(?i)\$\s*([0-9][0-9,]*(?:\.[0-9]{1,2})?)\s*(?:` + namePattern + `)\s+(?:fee|charge|cost)\b`)
	return after, before
}

// applicationFeeCheck recognizes commonly risky fee patterns without treating
// any single one as proof of wrongdoing: a stated holding fee, or a named fee
// above its normal ceiling. A fee far beyond that ceiling reads as more
// urgent in its detail text, but a fee amount alone never promotes this
// caution-level rule to a hard signal — see application_fee_check's
// parser-enforced "risk severity, never hard" constraint.
type applicationFeeCheck struct {
	name      string
	weight    float64
	scope     string
	threshold float64
	kind      string
}

func newApplicationFeeCheck(name string, r Rule, weight float64) (detect.Detector, error) {
	return &applicationFeeCheck{
		name: name, weight: weight, scope: r.Scope, threshold: r.ApplicationFeeHighThreshold, kind: r.FeeCheckKind,
	}, nil
}

func (a *applicationFeeCheck) Name() string { return a.name }

func (a *applicationFeeCheck) Evaluate(_ context.Context, listing domain.Listing) (domain.SignalResult, error) {
	text, present := scopeText(listing, a.scope)
	if !present {
		return domain.SignalResult{
			Name:    a.name,
			Skipped: domain.SkipMissingField,
			Details: []string{fmt.Sprintf("%s: listing has no %s text", a.name, a.scope)},
		}, nil
	}

	reasons := a.reasons(text)

	if len(reasons) == 0 {
		return domain.SignalResult{Name: a.name}, nil
	}
	return domain.SignalResult{
		Name:    a.name,
		Risk:    detect.Clamp(a.weight),
		Flags:   []string{a.name},
		Details: []string{fmt.Sprintf("%s: %s", a.name, strings.Join(reasons, "; "))},
	}, nil
}

func (a *applicationFeeCheck) reasons(text string) []string {
	reasons := []string{}
	if a.kind == "" || a.kind == FeeCheckKindCombined || a.kind == FeeCheckKindNonstandard {
		for _, pattern := range nonstandardFeePatterns {
			if matched := pattern.re.FindString(text); matched != "" {
				reasons = append(reasons, fmt.Sprintf("mentions a nonstandard %s (%q)", pattern.label, strings.TrimSpace(matched)))
			}
		}
	}
	if a.kind == FeeCheckKindNonstandard {
		return reasons
	}

	if amount, found := highestFeeAmount(text, reApplicationFeeAfter, reApplicationFeeBefore); found && amount > a.threshold {
		severe := amount > a.threshold*severeFeeMultiplier
		reasons = append(reasons, feeReason("application fee", amount, a.threshold, severe))
	}
	for _, category := range extraFeeCategories {
		amount, found := highestFeeAmount(text, category.after, category.before)
		if !found || amount <= category.normal {
			continue
		}
		severe := amount > category.normal*severeFeeMultiplier
		reasons = append(reasons, feeReason(category.label, amount, category.normal, severe))
	}
	return reasons
}

func feeReason(label string, amount, normal float64, severe bool) string {
	if severe {
		return fmt.Sprintf("%s of %s is far above the typical %s range — unusually high", label, formatDollars(amount), formatDollars(normal))
	}
	return fmt.Sprintf("%s of %s exceeds the %s caution threshold", label, formatDollars(amount), formatDollars(normal))
}

func highestFeeAmount(text string, after, before *regexp.Regexp) (float64, bool) {
	highest := 0.0
	found := false
	for _, re := range []*regexp.Regexp{after, before} {
		for _, match := range re.FindAllStringSubmatch(text, -1) {
			amount, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
			if err != nil || amount < 0 {
				continue
			}
			if !found || amount > highest {
				highest, found = amount, true
			}
		}
	}
	return highest, found
}

func formatDollars(amount float64) string {
	if amount == float64(int64(amount)) {
		return fmt.Sprintf("$%.0f", amount)
	}
	return fmt.Sprintf("$%.2f", amount)
}
