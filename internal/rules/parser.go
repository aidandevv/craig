package rules

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// Parse reads and validates a rule set. Validation is strict and happens once,
// at load, so a malformed rule fails before any listing is analyzed rather
// than silently never matching.
func Parse(data []byte) (RuleSet, error) {
	var set RuleSet
	if err := yaml.Unmarshal(data, &set); err != nil {
		return RuleSet{}, fmt.Errorf("rules: %w", err)
	}
	if err := set.validate(); err != nil {
		return RuleSet{}, err
	}
	return set, nil
}

// Load reads a rule set from disk.
func Load(path string) (RuleSet, error) {
	data, err := os.ReadFile(path)
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
	default:
		return fail("unknown rule type %q (want pattern_match, contact_check or image_analysis)", r.Type)
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
