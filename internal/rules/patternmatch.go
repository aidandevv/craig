package rules

import (
	"context"
	"fmt"
	"regexp"

	"github.com/aidandevv/craig-extension/internal/detect"
	"github.com/aidandevv/craig-extension/internal/domain"
)

// patternMatch is the workhorse rule type: regular expressions over selected
// listing text. It reports which pattern matched, because a score without a
// reason is not actionable.
type patternMatch struct {
	name     string
	weight   float64
	hard     bool
	scope    string
	compiled []*regexp.Regexp
	sources  []string // original pattern text, kept for the explanation
}

// newPatternMatch compiles a rule. Every regex is compiled once here rather
// than per evaluation, and a bad pattern fails the load rather than silently
// never matching.
func newPatternMatch(name string, r Rule, weight float64) (detect.Detector, error) {
	patterns, literal, err := patternSources(r)
	if err != nil {
		return nil, fmt.Errorf("rule %q: %w", name, err)
	}

	d := &patternMatch{name: name, weight: weight, hard: r.Hard, scope: r.Scope, sources: patterns}
	for i, source := range patterns {
		expr := source
		if literal {
			expr = regexp.QuoteMeta(expr)
		}
		if r.Insensitive() {
			expr = "(?i)" + expr
		}
		compiled, err := regexp.Compile(expr)
		if err != nil {
			return nil, fmt.Errorf("rule %q: pattern %d (%q): %w", name, i, source, err)
		}
		d.compiled = append(d.compiled, compiled)
	}
	return d, nil
}

// patternSources resolves a rule's patterns and reports whether they are
// literal text. Preset patterns are always regular expressions; custom
// patterns are literal unless the rule opts into regex.
func patternSources(r Rule) (patterns []string, literal bool, err error) {
	if r.Match.Preset != "" {
		preset, err := LookupPreset(r.Match.Preset)
		if err != nil {
			return nil, false, err
		}
		return preset.Patterns, false, nil
	}
	return r.Match.Custom, !r.Match.Regex, nil
}

func (p *patternMatch) Name() string { return p.name }

func (p *patternMatch) Evaluate(_ context.Context, listing domain.Listing) (domain.SignalResult, error) {
	haystack, present := scopeText(listing, p.scope)
	if !present {
		return domain.SignalResult{
			Name:    p.name,
			Skipped: domain.SkipMissingField,
			Details: []string{fmt.Sprintf("%s: listing has no %s text", p.name, p.scope)},
		}, nil
	}

	var hits []string
	for i, re := range p.compiled {
		if re.MatchString(haystack) {
			hits = append(hits, p.sources[i])
		}
	}
	if len(hits) == 0 {
		return domain.SignalResult{Name: p.name}, nil
	}

	return domain.SignalResult{
		Name:    p.name,
		Risk:    detect.Clamp(p.weight),
		Hard:    p.hard,
		Flags:   []string{p.name},
		Details: []string{fmt.Sprintf("%s: matched %q", p.name, hits[0])},
	}, nil
}
