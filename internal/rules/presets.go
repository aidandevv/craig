package rules

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed presets.yaml
var presetsYAML []byte

//go:embed default_rules.yaml
var defaultRulesYAML []byte

// Preset is a named group of patterns shipped with the binary. Presets use the
// same machinery as custom rules, so a user can inspect one and fork it.
type Preset struct {
	Description string   `yaml:"description" json:"description"`
	Patterns    []string `yaml:"patterns" json:"patterns"`
}

// DefaultRulesYAML returns a copy of the embedded starter rules. Configuration
// initialization uses it to create the first editable rules.yaml without
// making a user copy content from the binary or documentation.
func DefaultRulesYAML() []byte { return append([]byte(nil), defaultRulesYAML...) }

var (
	presetsOnce sync.Once
	presets     map[string]Preset
)

// Presets returns the embedded library. A malformed presets.yaml is a build
// defect, not a user error, so it panics rather than degrading silently.
func Presets() map[string]Preset {
	presetsOnce.Do(func() {
		if err := yaml.Unmarshal(presetsYAML, &presets); err != nil {
			panic("rules: embedded presets.yaml is malformed: " + err.Error())
		}
	})
	return presets
}

// LookupPreset resolves a preset by name, listing the alternatives when the
// name is wrong — a misremembered preset name is the likeliest authoring error.
func LookupPreset(name string) (Preset, error) {
	if preset, ok := Presets()[name]; ok {
		return preset, nil
	}
	available := make([]string, 0, len(Presets()))
	for key := range Presets() {
		available = append(available, key)
	}
	sort.Strings(available)
	return Preset{}, fmt.Errorf("unknown preset %q (available: %s)", name, strings.Join(available, ", "))
}

// DefaultRuleSet returns the embedded starter rules, allowing useful offline
// analysis before a user has created a configuration file.
func DefaultRuleSet() (RuleSet, error) {
	set, err := Parse(defaultRulesYAML)
	if err != nil {
		return RuleSet{}, fmt.Errorf("embedded default rules: %w", err)
	}
	return set, nil
}

// MigrateApplicationFeeRule adds the two default fee cautions to older user
// rule files. It upgrades the old embedded combined default, but leaves a
// hand-authored combined rule untouched so custom policy is never overwritten.
func MigrateApplicationFeeRule(set *RuleSet) (bool, error) {
	if set == nil {
		return false, fmt.Errorf("rules: cannot migrate a nil rule set")
	}
	defaults, err := DefaultRuleSet()
	if err != nil {
		return false, err
	}
	if set.Rules == nil {
		set.Rules = map[string]Rule{}
	}
	changed := false
	if legacy, exists := set.Rules["application_fee_details"]; exists && isLegacyDefaultFeeRule(legacy) {
		delete(set.Rules, "application_fee_details")
		changed = true
	}
	if _, legacyExists := set.Rules["application_fee_details"]; legacyExists {
		return changed, nil
	}
	for _, name := range []string{"nonstandard_rental_fee", "high_standard_rental_fee"} {
		if _, exists := set.Rules[name]; exists {
			continue
		}
		set.Rules[name] = defaults.Rules[name]
		changed = true
	}
	return changed, nil
}

func isLegacyDefaultFeeRule(rule Rule) bool {
	return rule.Type == TypeApplicationFeeCheck && rule.Severity == SeverityRisk && rule.Scope == ScopeWholePost &&
		rule.Description == "Holding fee or unusually high rental fee (application, admin, processing, credit check, and similar)" &&
		rule.FeeCheckKind == "" && rule.ApplicationFeeHighThreshold == 100 && rule.Weight == nil && !rule.Hard && rule.Enabled == nil
}

// MigrateMarketRentRule adds the bundled HUD market-rent rule to an older
// rules file when its key is absent. Existing custom policy, including a
// deliberately disabled version, is never replaced.
func MigrateMarketRentRule(set *RuleSet) (bool, error) {
	if set == nil {
		return false, fmt.Errorf("rules: cannot migrate a nil rule set")
	}
	if _, exists := set.Rules["market_rent_below_hud"]; exists {
		return false, nil
	}
	defaults, err := DefaultRuleSet()
	if err != nil {
		return false, err
	}
	if set.Rules == nil {
		set.Rules = map[string]Rule{}
	}
	set.Rules["market_rent_below_hud"] = defaults.Rules["market_rent_below_hud"]
	return true, nil
}

// MigrateRentPriceMismatchRule adds the title-versus-page-price consistency
// check to older editable rule files without changing a user's existing rule.
func MigrateRentPriceMismatchRule(set *RuleSet) (bool, error) {
	if set == nil {
		return false, fmt.Errorf("rules: cannot migrate a nil rule set")
	}
	if _, exists := set.Rules["rent_price_mismatch"]; exists {
		return false, nil
	}
	defaults, err := DefaultRuleSet()
	if err != nil {
		return false, err
	}
	if set.Rules == nil {
		set.Rules = map[string]Rule{}
	}
	set.Rules["rent_price_mismatch"] = defaults.Rules["rent_price_mismatch"]
	return true, nil
}
