package rules

import (
	"strings"
	"testing"
)

const minimalValid = `
version: "1.0"
severities:
  red: 0.45
  risk: 0.20
  green: 0.0
risk_bands:
  caution: 0.30
  elevated: 0.55
  high: 0.75
rules:
  gift_card_payment:
    type: pattern_match
    scope: whole_post
    severity: red
    match:
      preset: payment_no_recourse
`

func TestParseAcceptsMinimalRuleSet(t *testing.T) {
	got, err := Parse([]byte(minimalValid))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Version != "1.0" {
		t.Errorf("got version %q, want 1.0", got.Version)
	}
	rule, ok := got.Rules["gift_card_payment"]
	if !ok {
		t.Fatal("rule gift_card_payment missing")
	}
	if !rule.IsEnabled() {
		t.Error("a rule with no explicit enabled field must default to enabled")
	}
	if rule.Type != TypePatternMatch {
		t.Errorf("got type %q, want %q", rule.Type, TypePatternMatch)
	}
}

func TestParseRejectsInvalidRuleSets(t *testing.T) {
	cases := []struct {
		name      string
		yaml      string
		wantInErr string
	}{
		{
			name:      "unknown rule type",
			yaml:      replaceLine(minimalValid, "    type: pattern_match", "    type: telepathy"),
			wantInErr: "gift_card_payment",
		},
		{
			name:      "unknown scope",
			yaml:      replaceLine(minimalValid, "    scope: whole_post", "    scope: sidebar"),
			wantInErr: "sidebar",
		},
		{
			name:      "unknown severity",
			yaml:      replaceLine(minimalValid, "    severity: red", "    severity: chartreuse"),
			wantInErr: "chartreuse",
		},
		{
			name:      "green severity given a weight",
			yaml:      replaceLine(minimalValid, "  green: 0.0", "  green: 0.25"),
			wantInErr: "green",
		},
		{
			name:      "risk bands out of order",
			yaml:      replaceLine(minimalValid, "  elevated: 0.55", "  elevated: 0.10"),
			wantInErr: "ascending",
		},
		{
			name:      "pattern_match with no match source",
			yaml:      strings.Replace(minimalValid, "    match:\n      preset: payment_no_recourse\n", "", 1),
			wantInErr: "gift_card_payment",
		},
		{
			name: "pattern_match with both match sources",
			yaml: strings.Replace(minimalValid,
				"      preset: payment_no_recourse",
				"      preset: payment_no_recourse\n      custom: [\"wire\"]", 1),
			wantInErr: "exactly one",
		},
		{
			name:      "missing version",
			yaml:      replaceLine(minimalValid, `version: "1.0"`, `version: ""`),
			wantInErr: "version",
		},
		{
			name: "application fee check must be caution-only",
			yaml: minimalValid + `
  application_fee_details:
    type: application_fee_check
    scope: whole_post
    severity: red
    application_fee_high_threshold: 100
`,
			wantInErr: "application_fee_check must use risk severity",
		},
		{
			name: "application fee check needs a threshold",
			yaml: minimalValid + `
  application_fee_details:
    type: application_fee_check
    scope: whole_post
    severity: risk
`,
			wantInErr: "application_fee_high_threshold",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.yaml))
			if err == nil {
				t.Fatal("expected a validation error, got none")
			}
			if !strings.Contains(err.Error(), tc.wantInErr) {
				t.Errorf("error %q does not mention %q", err, tc.wantInErr)
			}
		})
	}
}

func TestParseRejectsGreenRuleWithExplicitWeight(t *testing.T) {
	y := minimalValid + `
  direct_phone_listed:
    type: contact_check
    scope: whole_post
    severity: green
    weight: 0.30
    checks: [direct_phone_present]
`
	_, err := Parse([]byte(y))
	if err == nil {
		t.Fatal("expected an error: a green rule must not carry a weight")
	}
	if !strings.Contains(err.Error(), "direct_phone_listed") {
		t.Errorf("error %q does not name the offending rule", err)
	}
}

func TestResolveWeightUsesSeverityThenOverride(t *testing.T) {
	set, err := Parse([]byte(minimalValid))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rule := set.Rules["gift_card_payment"]
	if got := set.Severities.WeightFor(rule); got != 0.45 {
		t.Errorf("got %v, want the red severity default 0.45", got)
	}

	override := 0.28
	rule.Weight = &override
	if got := set.Severities.WeightFor(rule); got != 0.28 {
		t.Errorf("got %v, want the explicit override 0.28", got)
	}

	green := Rule{Severity: SeverityGreen}
	if got := set.Severities.WeightFor(green); got != 0 {
		t.Errorf("a green rule must weigh 0, got %v", got)
	}
}

// replaceLine swaps one exact line, failing loudly if it is not present.
func replaceLine(doc, old, new string) string {
	if !strings.Contains(doc, old) {
		panic("test fixture does not contain: " + old)
	}
	return strings.Replace(doc, old, new, 1)
}
