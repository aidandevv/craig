package rules

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aidandevv/craig-extension/internal/detect"
	"github.com/aidandevv/craig-extension/internal/domain"
)

const matchContextWords = 4

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

func (p *patternMatch) Evaluate(ctx context.Context, listing domain.Listing) (domain.SignalResult, error) {
	haystack, present := scopeText(listing, p.scope)
	if !present {
		return domain.SignalResult{
			Name:    p.name,
			Skipped: domain.SkipMissingField,
			Details: []string{fmt.Sprintf("%s: listing has no %s text", p.name, p.scope)},
		}, nil
	}

	var hits []string
	var evidence []domain.TextMatchEvidence
	for i, re := range p.compiled {
		if err := ctx.Err(); err != nil {
			return domain.SignalResult{}, err
		}
		if location := re.FindStringIndex(haystack); location != nil {
			hits = append(hits, p.sources[i])
			evidence = append(evidence, textMatchEvidence(p.name, haystack, location))
		}
	}
	if len(hits) == 0 {
		return domain.SignalResult{Name: p.name}, nil
	}

	return domain.SignalResult{
		Name:        p.name,
		Risk:        detect.Clamp(p.weight),
		Hard:        p.hard,
		Flags:       []string{p.name},
		Details:     []string{fmt.Sprintf("%s: matched %q", p.name, hits[0])},
		TextMatches: evidence,
	}, nil
}

// textMatchEvidence keeps the exact regex hit and a small, whitespace-normalized
// word context. Listing text remains untrusted: this is structured data for a
// client to render as text, never HTML.
func textMatchEvidence(rule, text string, location []int) domain.TextMatchEvidence {
	return domain.TextMatchEvidence{
		Rule:   rule,
		Before: lastWords(text[:location[0]], matchContextWords),
		Match:  text[location[0]:location[1]],
		After:  firstWords(text[location[1]:], matchContextWords),
	}
}

func lastWords(text string, count int) string {
	words := strings.Fields(text)
	if len(words) > count {
		words = words[len(words)-count:]
	}
	return strings.Join(words, " ")
}

func firstWords(text string, count int) string {
	words := strings.Fields(text)
	if len(words) > count {
		words = words[:count]
	}
	return strings.Join(words, " ")
}
