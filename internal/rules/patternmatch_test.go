package rules

import (
	"context"
	"strings"
	"testing"

	"github.com/aidandevv/craig-extension/internal/detect"
	"github.com/aidandevv/craig-extension/internal/domain"
)

func mustPatternMatch(t *testing.T, name string, r Rule, weight float64) detect.Detector {
	t.Helper()
	d, err := newPatternMatch(name, r, weight)
	if err != nil {
		t.Fatalf("compile %s: %v", name, err)
	}
	return d
}

func TestScopeTextSelectsTheRightFields(t *testing.T) {
	listing := domain.Listing{
		Title:       "Sunny 2BR",
		Description: "Great light",
		Captions:    []string{"kitchen", "bathroom"},
	}
	cases := []struct {
		scope  string
		want   string
		wantOK bool
	}{
		{ScopeTitle, "Sunny 2BR", true},
		{ScopeDescription, "Great light", true},
		{ScopeCaption, "kitchen\nbathroom", true},
		{ScopeWholePost, "Sunny 2BR\nGreat light\nkitchen\nbathroom", true},
	}
	for _, tc := range cases {
		got, ok := scopeText(listing, tc.scope)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("scope %s: got (%q, %v), want (%q, %v)", tc.scope, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestScopeTextReportsAbsentText(t *testing.T) {
	empty := domain.Listing{Title: "Sunny 2BR"}
	if _, ok := scopeText(empty, ScopeDescription); ok {
		t.Error("an empty description must report absent, not empty-and-present")
	}
	if _, ok := scopeText(empty, ScopeCaption); ok {
		t.Error("absent captions must report absent")
	}
}

func TestPatternMatchFlagsAndExplains(t *testing.T) {
	rule := Rule{
		Type: TypePatternMatch, Scope: ScopeWholePost, Severity: SeverityRed,
		Match: Match{Custom: []string{"gift ?cards?", "western union"}, Regex: true},
	}
	got, err := mustPatternMatch(t, "gift_card_payment", rule, 0.45).
		Evaluate(context.Background(), domain.Listing{
			Description: "I can only take GIFT CARDS for the deposit",
		})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got.Risk != 0.45 {
		t.Errorf("got risk %v, want 0.45", got.Risk)
	}
	if len(got.Flags) != 1 || got.Flags[0] != "gift_card_payment" {
		t.Errorf("got flags %v, want [gift_card_payment]", got.Flags)
	}
	if len(got.Details) == 0 || !strings.HasPrefix(got.Details[0], "gift_card_payment: ") {
		t.Fatalf("details %v must be prefixed with the rule name so the assessment can attribute them", got.Details)
	}
	if !strings.Contains(got.Details[0], "gift ?cards?") {
		t.Errorf("details %v must name the pattern that matched", got.Details)
	}
	if len(got.TextMatches) != 1 {
		t.Fatalf("text matches = %+v, want one", got.TextMatches)
	}
	evidence := got.TextMatches[0]
	if evidence.Rule != "gift_card_payment" || evidence.Before != "I can only take" || evidence.Match != "GIFT CARDS" || evidence.After != "for the deposit" {
		t.Errorf("text match evidence = %+v", evidence)
	}
}

func TestPatternMatchRespectsScope(t *testing.T) {
	rule := Rule{
		Type: TypePatternMatch, Scope: ScopeTitle, Severity: SeverityRed,
		Match: Match{Custom: []string{"urgent"}, Regex: true},
	}
	detector := mustPatternMatch(t, "urgency", rule, 0.45)

	inBody, err := detector.Evaluate(context.Background(), domain.Listing{
		Title: "Sunny 2BR", Description: "urgent sale",
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if inBody.Risk != 0 {
		t.Errorf("a title-scoped rule must ignore the description, got risk %v", inBody.Risk)
	}

	inTitle, err := detector.Evaluate(context.Background(), domain.Listing{Title: "urgent 2BR"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if inTitle.Risk != 0.45 {
		t.Errorf("got risk %v, want 0.45", inTitle.Risk)
	}
}

func TestPatternMatchReportsMissingScopeTextAsNotEvaluated(t *testing.T) {
	rule := Rule{
		Type: TypePatternMatch, Scope: ScopeDescription, Severity: SeverityRed,
		Match: Match{Custom: []string{"urgent"}, Regex: true},
	}
	got, err := mustPatternMatch(t, "urgency", rule, 0.45).
		Evaluate(context.Background(), domain.Listing{Title: "no body text here"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got.Skipped != domain.SkipMissingField {
		t.Errorf("got skipped %q, want %q — a rule with nothing to read never passes silently",
			got.Skipped, domain.SkipMissingField)
	}
}

func TestPatternMatchTreatsNonRegexCustomPatternsAsLiterals(t *testing.T) {
	rule := Rule{
		Type: TypePatternMatch, Scope: ScopeWholePost, Severity: SeverityRisk,
		Match: Match{Custom: []string{"price (negotiable)"}, Regex: false},
	}
	detector := mustPatternMatch(t, "literal", rule, 0.20)

	hit, err := detector.Evaluate(context.Background(), domain.Listing{
		Description: "price (negotiable) for the right tenant",
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if hit.Risk != 0.20 {
		t.Errorf("literal pattern did not match its exact text, got risk %v", hit.Risk)
	}
}

func TestPatternMatchHonoursCaseSensitivity(t *testing.T) {
	sensitive := false
	rule := Rule{
		Type: TypePatternMatch, Scope: ScopeWholePost, Severity: SeverityRisk,
		CaseInsensitive: &sensitive,
		Match:           Match{Custom: []string{"URGENT"}, Regex: true},
	}
	got, err := mustPatternMatch(t, "shouty", rule, 0.20).
		Evaluate(context.Background(), domain.Listing{Description: "urgent"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got.Risk != 0 {
		t.Errorf("case-sensitive rule matched the wrong case, got risk %v", got.Risk)
	}
}

func TestPatternMatchResolvesPresets(t *testing.T) {
	rule := Rule{
		Type: TypePatternMatch, Scope: ScopeWholePost, Severity: SeverityRed,
		Match: Match{Preset: "payment_no_recourse"},
	}
	got, err := mustPatternMatch(t, "payment", rule, 0.45).
		Evaluate(context.Background(), domain.Listing{Description: "send it by western union"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got.Risk != 0.45 {
		t.Errorf("preset did not match, got risk %v", got.Risk)
	}
}

func TestPrepaymentBeforeAccessPresetIsCautionOnlyAndAvoidsSafetyAdvice(t *testing.T) {
	rule := Rule{
		Type: TypePatternMatch, Scope: ScopeWholePost, Severity: SeverityRisk,
		Match: Match{Preset: "prepayment_before_access"},
	}
	detector := mustPatternMatch(t, "prepayment_before_access", rule, .20)
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"pay before tour", "Please pay before your tour.", true},
		{"deposit for lockbox code", "Send the deposit for a lockbox code.", true},
		{"fee before keys", "The fee is due before keys are released.", true},
		{"safety advice", "Tour before paying any deposit; no fees are due before keys.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := detector.Evaluate(context.Background(), domain.Listing{Description: tc.text})
			if err != nil {
				t.Fatal(err)
			}
			matched := len(got.Flags) == 1
			if matched != tc.want {
				t.Errorf("matched=%v result=%+v", matched, got)
			}
			if matched && (got.Risk != .20 || got.Hard) {
				t.Errorf("caution result = %+v, want risk .20 and not hard", got)
			}
		})
	}
}

func TestPatternMatchRejectsBadRegexAtCompileTime(t *testing.T) {
	rule := Rule{
		Type: TypePatternMatch, Scope: ScopeWholePost, Severity: SeverityRed,
		Match: Match{Custom: []string{"ok", "wire (the ?(deposit"}, Regex: true},
	}
	_, err := newPatternMatch("broken", rule, 0.45)
	if err == nil {
		t.Fatal("expected a compile error for an invalid regex")
	}
	for _, want := range []string{"broken", "1", "wire (the ?(deposit"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q must mention %q (rule, pattern index, pattern text)", err, want)
		}
	}
}

func TestPatternMatchSetsHardFlag(t *testing.T) {
	rule := Rule{
		Type: TypePatternMatch, Scope: ScopeWholePost, Severity: SeverityRed, Hard: true,
		Match: Match{Custom: []string{"western union"}, Regex: true},
	}
	got, err := mustPatternMatch(t, "payment", rule, 0.45).
		Evaluate(context.Background(), domain.Listing{Description: "western union only"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !got.Hard {
		t.Error("a hard rule that matched must set Hard")
	}
}
