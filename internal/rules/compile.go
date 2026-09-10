package rules

import (
	"fmt"
	"sort"

	"github.com/aidandevv/craig-extension/internal/detect"
	"github.com/aidandevv/craig-extension/internal/domain"
	"github.com/aidandevv/craig-extension/internal/signals"
)

// Deps are the runtime collaborators a rule set may need. A nil or unconfigured
// Vision is normal, not an error: those rules are reported unavailable so the
// user can see the tool ran without them.
type Deps struct {
	Vision *signals.Vision
}

// Compiled is an executable rule set. Coverage connects a shared image
// detector's result back to each rule it evaluated.
type Compiled struct {
	Detectors   []detect.Detector
	Coverage    map[string][]string
	Unavailable map[string]string
	Rules       map[string]Rule
	Severities  Severities
	Bands       RiskBands
}

// MatchSpec is one image rule's contribution to a shared provider call.
type MatchSpec struct {
	Rule   string
	Tokens []string
	Weight float64
	Hard   bool
}

type factory func(name string, r Rule, weight float64) (detect.Detector, error)

var registry = map[string]factory{
	TypePatternMatch: newPatternMatch,
	TypeContactCheck: newContactCheck,
}

// Compile turns a validated rule set into detectors. Rules are processed in
// sorted order so a broken rule set always reports the same first error.
func Compile(set RuleSet, deps Deps) (Compiled, error) {
	compiled := Compiled{
		Coverage: map[string][]string{}, Unavailable: map[string]string{}, Rules: map[string]Rule{},
		Severities: set.Severities, Bands: set.RiskBands,
	}

	names := make([]string, 0, len(set.Rules))
	for name, rule := range set.Rules {
		if rule.IsEnabled() {
			names = append(names, name)
			compiled.Rules[name] = rule
		}
	}
	sort.Strings(names)

	for _, name := range names {
		rule := set.Rules[name]
		if rule.Type == TypeImageAnalysis {
			continue
		}
		build, ok := registry[rule.Type]
		if !ok {
			return Compiled{}, fmt.Errorf("rule %q: no compiler for type %q", name, rule.Type)
		}
		detector, err := build(name, rule, set.Severities.WeightFor(rule))
		if err != nil {
			return Compiled{}, err
		}
		compiled.Detectors = append(compiled.Detectors, detector)
		compiled.Coverage[name] = []string{name}
	}
	if err := compileImageRules(set, deps, &compiled); err != nil {
		return Compiled{}, err
	}
	return compiled, nil
}

// compileImageRules coalesces every image rule sharing a method into a single
// detector. This preserves the Vision API budget: one provider feature per
// image, rather than one for every matching YAML rule.
func compileImageRules(set RuleSet, deps Deps, compiled *Compiled) error {
	groups := imageGroups(set)
	if len(groups) == 0 {
		return nil
	}
	if deps.Vision == nil || !deps.Vision.Enabled() {
		for _, specs := range groups {
			for _, spec := range specs {
				compiled.Unavailable[spec.Rule] = domain.SkipNoAPIKey
			}
		}
		return nil
	}

	methods := make([]string, 0, len(groups))
	for method := range groups {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	for _, method := range methods {
		specs := groups[method]
		matchGroups := make([]signals.MatchGroup, 0, len(specs))
		covered := make([]string, 0, len(specs))
		for _, spec := range specs {
			matchGroups = append(matchGroups, signals.MatchGroup{
				Rule: spec.Rule, Tokens: spec.Tokens, Weight: spec.Weight, Hard: spec.Hard,
			})
			covered = append(covered, spec.Rule)
		}

		switch method {
		case MethodReverseSearch:
			const name = "reverse_image"
			compiled.Detectors = append(compiled.Detectors,
				signals.NewReverseImage(deps.Vision, signals.ReverseImageOptions{Name: name, Groups: matchGroups}))
			compiled.Coverage[name] = covered
		case MethodOCR:
			const name = "ocr"
			compiled.Detectors = append(compiled.Detectors,
				signals.NewOCR(deps.Vision, signals.OCROptions{Name: name, Groups: matchGroups}))
			compiled.Coverage[name] = covered
		default:
			return fmt.Errorf("no compiler for image_analysis method %q", method)
		}
	}
	return nil
}

// imageGroups buckets enabled image rules by method, in sorted rule order.
func imageGroups(set RuleSet) map[string][]MatchSpec {
	names := make([]string, 0, len(set.Rules))
	for name, rule := range set.Rules {
		if rule.IsEnabled() && rule.Type == TypeImageAnalysis {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	groups := map[string][]MatchSpec{}
	for _, name := range names {
		rule := set.Rules[name]
		tokens := rule.FlagsMatching
		if rule.Method == MethodOCR {
			tokens = rule.TextMatching
		}
		groups[rule.Method] = append(groups[rule.Method], MatchSpec{
			Rule: name, Tokens: tokens, Weight: set.Severities.WeightFor(rule), Hard: rule.Hard,
		})
	}
	return groups
}
