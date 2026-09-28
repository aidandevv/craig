package rules

import "sort"

// RuleSchema describes what the rule builder may offer. The daemon serves it
// over HTTP and the browser build returns it from WebAssembly.
type RuleSchema struct {
	RuleTypes    []string          `json:"rule_types"`
	Scopes       []string          `json:"scopes"`
	Severities   []string          `json:"severities"`
	ImageMethods []string          `json:"image_methods"`
	Presets      map[string]Preset `json:"presets"`
	PresetNames  []string          `json:"preset_names"`
}

func Schema() RuleSchema {
	presets := Presets()
	names := make([]string, 0, len(presets))
	for name := range presets {
		names = append(names, name)
	}
	sort.Strings(names)
	return RuleSchema{
		RuleTypes:    []string{TypePatternMatch, TypeContactCheck, TypeImageAnalysis, TypeApplicationFeeCheck, TypeMarketRentCheck, TypeRentPriceMismatch},
		Scopes:       []string{ScopeTitle, ScopeDescription, ScopeCaption, ScopeWholePost},
		Severities:   []string{SeverityRed, SeverityRisk, SeverityGreen},
		ImageMethods: []string{MethodReverseSearch, MethodOCR},
		Presets:      presets,
		PresetNames:  names,
	}
}
