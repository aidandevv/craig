// Package rules turns a user-editable YAML rule set into detectors.
//
// Rule kinds are a closed set. That is a deliberate constraint rather than a
// limitation: a visual rule builder needs an enumerable schema, and a typed
// rule can explain exactly what it matched in a way an arbitrary expression
// cannot.
package rules

// Rule types.
const (
	TypePatternMatch        = "pattern_match"
	TypeContactCheck        = "contact_check"
	TypeImageAnalysis       = "image_analysis"
	TypeApplicationFeeCheck = "application_fee_check"
	TypeMarketRentCheck     = "market_rent_check"
	TypeRentPriceMismatch   = "rent_price_mismatch"
)

// Scopes select which listing text a rule reads.
const (
	ScopeTitle       = "title"
	ScopeDescription = "description"
	ScopeCaption     = "caption"
	ScopeWholePost   = "whole_post"
)

// Severities. Green is informational: it is displayed but never scored,
// because anything that lowers a score is something a scammer writes on purpose.
const (
	SeverityRed   = "red"
	SeverityRisk  = "risk"
	SeverityGreen = "green"
)

// Image analysis methods.
const (
	MethodReverseSearch = "reverse_search"
	MethodOCR           = "ocr"
)

// RuleSet is a parsed rules.yaml.
type RuleSet struct {
	Version    string          `yaml:"version" json:"version"`
	Severities Severities      `yaml:"severities" json:"severities"`
	RiskBands  RiskBands       `yaml:"risk_bands" json:"risk_bands"`
	Rules      map[string]Rule `yaml:"rules" json:"rules"`
}

// Severities maps a severity tier to the weight a matching rule contributes.
type Severities struct {
	Red   float64 `yaml:"red" json:"red"`
	Risk  float64 `yaml:"risk" json:"risk"`
	Green float64 `yaml:"green" json:"green"`
}

// WeightFor resolves a rule's contribution: an explicit weight wins, otherwise
// the severity default. Green always resolves to zero regardless of either,
// so the guarantee cannot be defeated by an edited or imported rule set.
func (s Severities) WeightFor(r Rule) float64 {
	if r.Severity == SeverityGreen {
		return 0
	}
	if r.Weight != nil {
		return *r.Weight
	}
	switch r.Severity {
	case SeverityRed:
		return s.Red
	case SeverityRisk:
		return s.Risk
	}
	return 0
}

// RiskBands are the lower bounds of each band above "low".
type RiskBands struct {
	Caution  float64 `yaml:"caution" json:"caution"`
	Elevated float64 `yaml:"elevated" json:"elevated"`
	High     float64 `yaml:"high" json:"high"`
}

// Rule is one check. Fields are grouped by the type that uses them; a field
// belonging to another type is a validation error, not silently ignored.
type Rule struct {
	Type        string `yaml:"type" json:"type"`
	Severity    string `yaml:"severity" json:"severity"`
	Description string `yaml:"description" json:"description,omitempty"`

	Enabled *bool    `yaml:"enabled" json:"enabled,omitempty"` // nil means enabled
	Weight  *float64 `yaml:"weight" json:"weight,omitempty"`   // nil means use the severity default
	Hard    bool     `yaml:"hard" json:"hard,omitempty"`

	// pattern_match, contact_check, and application_fee_check
	Scope           string `yaml:"scope" json:"scope,omitempty"`
	CaseInsensitive *bool  `yaml:"case_insensitive" json:"case_insensitive,omitempty"` // nil means true

	// pattern_match
	Match Match `yaml:"match" json:"match,omitempty"`

	// contact_check
	Checks     []string `yaml:"checks" json:"checks,omitempty"`
	RequireAll bool     `yaml:"require_all" json:"require_all,omitempty"`

	// image_analysis
	Method        string   `yaml:"method" json:"method,omitempty"`
	FlagsMatching []string `yaml:"flags_matching" json:"flags_matching,omitempty"`
	TextMatching  []string `yaml:"text_matching" json:"text_matching,omitempty"`

	// application_fee_check
	ApplicationFeeHighThreshold float64 `yaml:"application_fee_high_threshold" json:"application_fee_high_threshold,omitempty"`
	FeeCheckKind                string  `yaml:"fee_check_kind" json:"fee_check_kind,omitempty"`

	// market_rent_check
	MarketRentLowRatio float64 `yaml:"market_rent_low_ratio" json:"market_rent_low_ratio,omitempty"`

	// rent_price_mismatch
	// RentPriceMismatchRatio is the minimum relative difference between the
	// page's structured price and a dollar amount advertised in the title.
	RentPriceMismatchRatio float64 `yaml:"rent_price_mismatch_ratio" json:"rent_price_mismatch_ratio,omitempty"`
}

// IsEnabled reports whether the rule runs. Rules are enabled unless disabled.
func (r Rule) IsEnabled() bool { return r.Enabled == nil || *r.Enabled }

// Insensitive reports whether matching ignores case. Defaults to true.
func (r Rule) Insensitive() bool { return r.CaseInsensitive == nil || *r.CaseInsensitive }

// Match is a pattern_match rule's source of patterns: exactly one of a curated
// preset or a user-supplied list.
type Match struct {
	Preset string   `yaml:"preset" json:"preset,omitempty"`
	Custom []string `yaml:"custom" json:"custom,omitempty"`
	Regex  bool     `yaml:"regex" json:"regex,omitempty"` // custom patterns only; presets are always regex
}
