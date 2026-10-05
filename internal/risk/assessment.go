package risk

import (
	"sort"
	"strings"
	"time"

	"github.com/aidandevv/craig/internal/domain"
	"github.com/aidandevv/craig/internal/rules"
)

type Finding struct {
	Rule         string               `json:"rule"`
	Label        string               `json:"label"`
	Detail       string               `json:"detail,omitempty"`
	Weight       float64              `json:"weight,omitempty"`
	TextMatch    *TextMatchEvidence   `json:"text_match,omitempty"`
	ImageMatches []ImageMatchEvidence `json:"image_matches,omitempty"`
}

// TextMatchEvidence is the presentation-safe form of a text-rule match. The
// rule key is already carried by Finding, so it is intentionally not repeated.
type TextMatchEvidence struct {
	Before string `json:"before,omitempty"`
	Match  string `json:"match"`
	After  string `json:"after,omitempty"`
}

// ImageMatchEvidence is safe-to-render provenance for a reverse-image
// finding. The detector-only rule name is intentionally omitted at this API
// boundary because the finding already identifies its rule.
type ImageMatchEvidence struct {
	ListingImageURL string `json:"listing_image_url"`
	SourcePageURL   string `json:"source_page_url"`
	SourceImageURL  string `json:"source_image_url,omitempty"`
}
type NotEvaluated struct {
	Rule   string `json:"rule"`
	Reason string `json:"reason"`
}
type Coverage struct {
	Ran     int `json:"ran"`
	Enabled int `json:"enabled"`
}

// ImageCoverage reports successful photos and the oldest evidence timestamp.
type ImageCoverage struct {
	Signal    string    `json:"signal"`
	Checked   int       `json:"checked"`
	Total     int       `json:"total"`
	CheckedAt time.Time `json:"checked_at"`
}

// Assessment is the complete, explained verdict on one listing.
type Assessment struct {
	ImageCoverage    []ImageCoverage             `json:"image_coverage,omitempty"`
	ImageCandidates  []domain.ImageMatchEvidence `json:"image_candidates,omitempty"`
	RiskScore        float64                     `json:"risk_score"`
	RiskBand         string                      `json:"risk_band"`
	HardFlagged      bool                        `json:"hard_flagged"`
	Coverage         Coverage                    `json:"coverage"`
	HighRisk         []Finding                   `json:"high_risk"`
	PotentiallyRisky []Finding                   `json:"potentially_risky"`
	PositiveSignals  []Finding                   `json:"positive_signals"`
	PassedChecks     []string                    `json:"passed_checks"`
	NotEvaluated     []NotEvaluated              `json:"not_evaluated"`
	AnalysisTimeMS   int64                       `json:"analysis_time_ms"`
}

// Assess places every enabled rule in exactly one bucket. In particular, it
// never represents unavailable checks as passing; absence of evidence must not
// be presented as evidence of safety.
func Assess(results []domain.SignalResult, compiled rules.Compiled, elapsed time.Duration) Assessment {
	score, band, hard := Score(results, compiled.Bands)
	// Keep the JSON boundary stable for browser clients: nil Go slices encode as
	// null, while an assessment group is always conceptually a list (possibly
	// empty). Initializing every group avoids forcing clients to special-case
	// otherwise successful assessment responses.
	a := Assessment{
		RiskScore:        score,
		RiskBand:         band,
		HardFlagged:      hard,
		HighRisk:         []Finding{},
		PotentiallyRisky: []Finding{},
		PositiveSignals:  []Finding{},
		PassedChecks:     []string{},
		NotEvaluated:     []NotEvaluated{},
		AnalysisTimeMS:   elapsed.Milliseconds(),
	}
	notEvaluated := make(map[string]string, len(compiled.Unavailable))
	for rule, reason := range compiled.Unavailable {
		notEvaluated[rule] = reason
	}
	passed := make(map[string]bool)

	for _, result := range results {
		covered := compiled.Coverage[result.Name]
		if result.ImagesTotal > 0 {
			a.ImageCoverage = append(a.ImageCoverage, ImageCoverage{result.Name, result.ImagesChecked, result.ImagesTotal, result.CheckedAt})
		}
		a.ImageCandidates = append(a.ImageCandidates, result.ImageCandidates...)
		if len(covered) == 0 {
			continue
		}
		if result.Skipped != "" {
			for _, rule := range covered {
				notEvaluated[rule] = result.Skipped
				delete(passed, rule)
			}
			continue
		}
		matched := make(map[string]bool, len(result.Flags))
		for _, flag := range result.Flags {
			matched[flag] = true
		}
		for _, rule := range covered {
			delete(notEvaluated, rule)
			if matched[rule] {
				a.add(rule, compiled, detailFor(result, rule), textMatchFor(result, rule), imageMatchesFor(result, rule))
				continue
			}
			if result.ImagesChecked < result.ImagesTotal {
				notEvaluated[rule] = domain.SkipPartialImages
				continue
			}
			passed[rule] = true
		}
	}

	for rule := range passed {
		if _, unavailable := notEvaluated[rule]; !unavailable {
			a.PassedChecks = append(a.PassedChecks, rule)
		}
	}
	sort.Strings(a.PassedChecks)
	sort.Slice(a.HighRisk, func(i, j int) bool { return a.HighRisk[i].Rule < a.HighRisk[j].Rule })
	sort.Slice(a.PotentiallyRisky, func(i, j int) bool { return a.PotentiallyRisky[i].Rule < a.PotentiallyRisky[j].Rule })
	sort.Slice(a.PositiveSignals, func(i, j int) bool { return a.PositiveSignals[i].Rule < a.PositiveSignals[j].Rule })
	names := make([]string, 0, len(notEvaluated))
	for rule := range notEvaluated {
		names = append(names, rule)
	}
	sort.Strings(names)
	for _, rule := range names {
		a.NotEvaluated = append(a.NotEvaluated, NotEvaluated{Rule: rule, Reason: notEvaluated[rule]})
	}
	a.Coverage = Coverage{Enabled: len(compiled.Rules), Ran: len(compiled.Rules) - len(a.NotEvaluated)}
	return a
}

func (a *Assessment) add(name string, compiled rules.Compiled, detail string, textMatch *TextMatchEvidence, imageMatches []ImageMatchEvidence) {
	rule := compiled.Rules[name]
	finding := Finding{Rule: name, Label: label(name, rule), Detail: detail, Weight: compiled.Severities.WeightFor(rule), TextMatch: textMatch, ImageMatches: imageMatches}
	switch {
	case rule.Severity == rules.SeverityGreen:
		a.PositiveSignals = append(a.PositiveSignals, finding)
	case rule.Severity == rules.SeverityRed || rule.Hard:
		a.HighRisk = append(a.HighRisk, finding)
	default:
		a.PotentiallyRisky = append(a.PotentiallyRisky, finding)
	}
}

func textMatchFor(result domain.SignalResult, rule string) *TextMatchEvidence {
	for _, evidence := range result.TextMatches {
		if evidence.Rule == rule && evidence.Match != "" {
			return &TextMatchEvidence{Before: evidence.Before, Match: evidence.Match, After: evidence.After}
		}
	}
	return nil
}

func imageMatchesFor(result domain.SignalResult, rule string) []ImageMatchEvidence {
	seen := map[string]bool{}
	matches := []ImageMatchEvidence{}
	for _, match := range result.ImageMatches {
		if match.Rule != rule || match.ListingImageURL == "" || match.SourcePageURL == "" {
			continue
		}
		key := match.ListingImageURL + "\x00" + match.SourcePageURL + "\x00" + match.SourceImageURL
		if seen[key] {
			continue
		}
		seen[key] = true
		matches = append(matches, ImageMatchEvidence{
			ListingImageURL: match.ListingImageURL,
			SourcePageURL:   match.SourcePageURL,
			SourceImageURL:  match.SourceImageURL,
		})
	}
	return matches
}
func label(name string, rule rules.Rule) string {
	if strings.TrimSpace(rule.Description) != "" {
		return rule.Description
	}
	return name
}
func detailFor(result domain.SignalResult, rule string) string {
	prefix := rule + ": "
	for _, detail := range result.Details {
		if strings.HasPrefix(detail, prefix) {
			return strings.TrimPrefix(detail, prefix)
		}
	}
	return ""
}
