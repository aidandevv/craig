package rules

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// Parse reads and validates a rule set. Validation is strict and happens once,
// at load, so a malformed rule fails before any listing is analyzed rather
// than silently never matching.
func Parse(data []byte) (RuleSet, error) {
	var set RuleSet
	if len(data) > 1<<20 {
		return RuleSet{}, fmt.Errorf("rules: file exceeds size limit")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&set); err != nil {
		return RuleSet{}, fmt.Errorf("rules: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return RuleSet{}, fmt.Errorf("rules: trailing YAML document")
	}
	if err := set.validate(); err != nil {
		return RuleSet{}, err
	}
	return set, nil
}

// Validate checks a programmatically constructed rule set. API clients use
// this through the YAML round trip before a new rules file is accepted.
func Validate(set RuleSet) error { return set.validate() }

// Load reads a rule set from disk.
func Load(path string) (RuleSet, error) {
	file, err := os.Open(path)
	if err != nil {
		return RuleSet{}, fmt.Errorf("rules: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil {
		return RuleSet{}, fmt.Errorf("rules: %w", err)
	}
	set, err := Parse(data)
	if err != nil {
		return RuleSet{}, fmt.Errorf("%s: %w", path, err)
	}
	return set, nil
}

func (s RuleSet) validate() error {
	if len(s.Rules) > 256 {
		return fmt.Errorf("rules: at most 256 rules are allowed")
	}
	total := len(s.Version)
	entries := 0
	for name, rule := range s.Rules {
		total += len(name) + len(rule.Description) + len(rule.Type) + len(rule.Scope) + len(rule.Severity) + len(rule.Method) + len(rule.Match.Preset) + len(rule.FeeCheckKind)
		for _, values := range [][]string{rule.Match.Custom, rule.Checks, rule.FlagsMatching, rule.TextMatching} {
			entries += len(values)
			if entries > 1024 {
				return fmt.Errorf("rules: at most 1024 pattern and check entries are allowed")
			}
			if len(values) > 64 {
				return fmt.Errorf("rules: at most 64 entries per pattern or check list are allowed")
			}
			for _, value := range values {
				if len(value) > 4096 {
					return fmt.Errorf("rules: pattern or check exceeds size limit")
				}
				total += len(value)
			}
		}
		if len(name) > 128 || len(rule.Description) > 4096 || total > 256_000 {
			return fmt.Errorf("rules: rule data exceeds size limit")
		}
	}
	if total > 256_000 {
		return fmt.Errorf("rules: rule data exceeds size limit")
	}

	if s.Version == "" {
		return fmt.Errorf("rules: version is required")
	}
	if s.Severities.Green != 0 {
		return fmt.Errorf("rules: severities.green must be 0.0; green flags are informational and never change the score")
	}
	for name, weight := range map[string]float64{"red": s.Severities.Red, "risk": s.Severities.Risk} {
		if weight < 0 || weight > 1 {
			return fmt.Errorf("rules: severities.%s must be in [0,1], got %v", name, weight)
		}
	}
	b := s.RiskBands
	if !(b.Caution > 0 && b.Caution < b.Elevated && b.Elevated < b.High && b.High <= 1) {
		return fmt.Errorf("rules: risk_bands must be ascending within (0,1]: caution %v, elevated %v, high %v",
			b.Caution, b.Elevated, b.High)
	}
	if len(s.Rules) == 0 {
		return fmt.Errorf("rules: at least one rule is required")
	}
	// Sorted so the same bad file always reports the same first error.
	names := make([]string, 0, len(s.Rules))
	for name := range s.Rules {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := s.Rules[name].validate(name); err != nil {
			return err
		}
	}
	return nil
}

func (r Rule) validate(name string) error {
	fail := func(format string, args ...any) error {
		return fmt.Errorf("rule %q: "+format, append([]any{name}, args...)...)
	}

	switch r.Severity {
	case SeverityRed, SeverityRisk:
	case SeverityGreen:
		if r.Weight != nil {
			return fail("a green rule must not set a weight; green flags never change the score")
		}
		if r.Hard {
			return fail("a green rule must not be hard")
		}
	default:
		return fail("unknown severity %q (want red, risk or green)", r.Severity)
	}
	if r.Weight != nil && (*r.Weight < 0 || *r.Weight > 1) {
		return fail("weight must be in [0,1], got %v", *r.Weight)
	}

	switch r.Type {
	case TypePatternMatch:
		if err := validateScope(r.Scope); err != nil {
			return fail("%w", err)
		}
		hasPreset, hasCustom := r.Match.Preset != "", len(r.Match.Custom) > 0
		if hasPreset == hasCustom {
			return fail("pattern_match needs exactly one of match.preset or match.custom")
		}
	case TypeContactCheck:
		if err := validateScope(r.Scope); err != nil {
			return fail("%w", err)
		}
		if len(r.Checks) == 0 {
			return fail("contact_check needs at least one entry in checks")
		}
		for _, check := range r.Checks {
			if !knownCheck(check) {
				return fail("unknown check %q", check)
			}
		}
	case TypeImageAnalysis:
		switch r.Method {
		case MethodReverseSearch:
			if len(r.FlagsMatching) == 0 {
				return fail("image_analysis with method reverse_search needs flags_matching")
			}
		case MethodOCR:
			if len(r.TextMatching) == 0 {
				return fail("image_analysis with method ocr needs text_matching")
			}
		default:
			return fail("unknown image_analysis method %q (want reverse_search or ocr)", r.Method)
		}
	case TypeApplicationFeeCheck:
		if err := validateScope(r.Scope); err != nil {
			return fail("%w", err)
		}
		if r.Severity != SeverityRisk || r.Hard {
			return fail("application_fee_check must use risk severity and cannot be hard")
		}
		switch r.FeeCheckKind {
		case "", FeeCheckKindCombined, FeeCheckKindHighStandard:
			if r.ApplicationFeeHighThreshold <= 0 {
				return fail("application_fee_check needs application_fee_high_threshold greater than zero")
			}
		case FeeCheckKindNonstandard:
			if r.ApplicationFeeHighThreshold != 0 {
				return fail("nonstandard application_fee_check must not set application_fee_high_threshold")
			}
		default:
			return fail("application_fee_check has unknown fee_check_kind %q (want nonstandard, high_standard, or combined)", r.FeeCheckKind)
		}
	case TypeMarketRentCheck:
		if r.Severity != SeverityRisk || r.Hard {
			return fail("market_rent_check must use risk severity and cannot be hard")
		}
		if r.MarketRentLowRatio <= 0 || r.MarketRentLowRatio >= 1 {
			return fail("market_rent_check needs market_rent_low_ratio within (0,1)")
		}
	case TypeRentPriceMismatch:
		if r.Severity != SeverityRisk || r.Hard {
			return fail("rent_price_mismatch must use risk severity and cannot be hard")
		}
		if r.RentPriceMismatchRatio <= 0 || r.RentPriceMismatchRatio >= 1 {
			return fail("rent_price_mismatch needs rent_price_mismatch_ratio within (0,1)")
		}
	default:
		return fail("unknown rule type %q (want pattern_match, contact_check, image_analysis, application_fee_check, market_rent_check or rent_price_mismatch)", r.Type)
	}
	return nil
}

func validateScope(scope string) error {
	switch scope {
	case ScopeTitle, ScopeDescription, ScopeCaption, ScopeWholePost:
		return nil
	}
	return fmt.Errorf("unknown scope %q (want title, description, caption or whole_post)", scope)
}
