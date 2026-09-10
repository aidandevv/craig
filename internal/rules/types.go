// Package rules turns a user-editable YAML rule set into detectors.
//
// Rule kinds are a closed set. That is a deliberate constraint rather than a
// limitation: a visual rule builder needs an enumerable schema, and a typed
// rule can explain exactly what it matched in a way an arbitrary expression
// cannot.
package rules

// Rule types.
const (
	TypePatternMatch  = "pattern_match"
	TypeContactCheck  = "contact_check"
	TypeImageAnalysis = "image_analysis"
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
	Version    string          `yaml:"version"`
	Severities Severities      `yaml:"severities"`
	RiskBands  RiskBands       `yaml:"risk_bands"`
	Rules      map[string]Rule `yaml:"rules"`
}

// Severities maps a severity tier to the weight a matching rule contributes.
type Severities struct {
	Red   float64 `yaml:"red"`
	Risk  float64 `yaml:"risk"`
	Green float64 `yaml:"green"`
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
	Caution  float64 `yaml:"caution"`
	Elevated float64 `yaml:"elevated"`
	High     float64 `yaml:"high"`
}

// Rule is one check. Fields are grouped by the type that uses them; a field
// belonging to another type is a validation error, not silently ignored.
type Rule struct {
	Type        string `yaml:"type"`
	Severity    string `yaml:"severity"`
	Description string `yaml:"description"`

	Enabled *bool    `yaml:"enabled"` // nil means enabled
	Weight  *float64 `yaml:"weight"`  // nil means use the severity default
	Hard    bool     `yaml:"hard"`

	// pattern_match and contact_check
	Scope           string `yaml:"scope"`
	CaseInsensitive *bool  `yaml:"case_insensitive"` // nil means true

	// pattern_match
	Match Match `yaml:"match"`

	// contact_check
	Checks     []string `yaml:"checks"`
	RequireAll bool     `yaml:"require_all"`

	// image_analysis
	Method        string   `yaml:"method"`
	FlagsMatching []string `yaml:"flags_matching"`
	TextMatching  []string `yaml:"text_matching"`
}

// IsEnabled reports whether the rule runs. Rules are enabled unless disabled.
func (r Rule) IsEnabled() bool { return r.Enabled == nil || *r.Enabled }

// Insensitive reports whether matching ignores case. Defaults to true.
func (r Rule) Insensitive() bool { return r.CaseInsensitive == nil || *r.CaseInsensitive }

// Match is a pattern_match rule's source of patterns: exactly one of a curated
// preset or a user-supplied list.
type Match struct {
	Preset string   `yaml:"preset"`
	Custom []string `yaml:"custom"`
	Regex  bool     `yaml:"regex"` // custom patterns only; presets are always regex
}
