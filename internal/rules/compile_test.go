package rules

import (
	"sort"
	"strings"
	"testing"

	"github.com/aidandevv/craig-extension/internal/domain"
)

const compileFixture = `
version: "1.0"
severities: {red: 0.45, risk: 0.20, green: 0.0}
risk_bands: {caution: 0.30, elevated: 0.55, high: 0.75}
rules:
  gift_card_payment:
    type: pattern_match
    scope: whole_post
    severity: red
    match: {preset: payment_no_recourse}
  contact_evasion:
    type: contact_check
    scope: whole_post
    severity: risk
    checks: [relay_only]
  disabled_rule:
    type: pattern_match
    scope: title
    severity: risk
    enabled: false
    match: {custom: ["never runs"]}
  application_fee_details:
    type: application_fee_check
    scope: whole_post
    severity: risk
    application_fee_high_threshold: 100
  market_rent_below_hud:
    type: market_rent_check
    severity: risk
    weight: 0.15
    market_rent_low_ratio: 0.55
  reverse_image_real_estate:
    type: image_analysis
    method: reverse_search
    severity: red
    hard: true
    weight: 0.60
    flags_matching: [zillow, redfin]
  stock_photos:
    type: image_analysis
    method: reverse_search
    severity: risk
    weight: 0.35
    flags_matching: [shutterstock]
  mls_watermark:
    type: image_analysis
    method: ocr
    severity: red
    hard: true
    weight: 0.30
    text_matching: ["multiple listing service"]
`

func compileFixtureSet(t *testing.T, deps Deps) Compiled {
	t.Helper()
	set, err := Parse([]byte(compileFixture))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	compiled, err := Compile(set, deps)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return compiled
}

func TestCompileExcludesDisabledRules(t *testing.T) {
	c := compileFixtureSet(t, Deps{})
	if _, ok := c.Rules["disabled_rule"]; ok {
		t.Error("disabled rule is executable")
	}
	for _, names := range c.Coverage {
		for _, name := range names {
			if name == "disabled_rule" {
				t.Error("disabled rule has coverage")
			}
		}
	}
	if _, ok := c.Unavailable["disabled_rule"]; ok {
		t.Error("disabled rule is unavailable instead of off")
	}
}

func TestCompileBuildsNonImageDetectorsAndCoverage(t *testing.T) {
	c := compileFixtureSet(t, Deps{})
	if got, want := detectorNames(c), []string{"application_fee_details", "contact_evasion", "gift_card_payment", "market_rent_below_hud"}; !sameStrings(got, want) {
		t.Errorf("detectors = %v, want %v", got, want)
	}
	for _, name := range []string{"contact_evasion", "gift_card_payment"} {
		if got := c.Coverage[name]; !sameStrings(got, []string{name}) {
			t.Errorf("coverage[%q] = %v", name, got)
		}
	}
}

func TestCompileMarksImageRulesUnavailableWithoutVision(t *testing.T) {
	c := compileFixtureSet(t, Deps{})
	for _, name := range []string{"reverse_image_real_estate", "stock_photos", "mls_watermark"} {
		if got := c.Unavailable[name]; got != domain.SkipNoAPIKey {
			t.Errorf("unavailable[%q] = %q", name, got)
		}
	}
}

func TestCompileCoalescesImageRulesByMethod(t *testing.T) {
	set, err := Parse([]byte(compileFixture))
	if err != nil {
		t.Fatal(err)
	}
	groups := imageGroups(set)
	reverse := groups[MethodReverseSearch]
	if len(reverse) != 2 {
		t.Fatalf("reverse rules = %d, want 2", len(reverse))
	}
	byRule := map[string]MatchSpec{}
	for _, spec := range reverse {
		byRule[spec.Rule] = spec
	}
	if got := byRule["reverse_image_real_estate"]; got.Weight != .60 || !got.Hard || !sameStrings(got.Tokens, []string{"zillow", "redfin"}) {
		t.Errorf("real-estate group = %+v", got)
	}
	if got := byRule["stock_photos"]; got.Weight != .35 || got.Hard {
		t.Errorf("stock group = %+v", got)
	}
	if got := groups[MethodOCR]; len(got) != 1 || got[0].Rule != "mls_watermark" {
		t.Errorf("ocr group = %+v", got)
	}
}

func TestCompileCarriesScoringConfiguration(t *testing.T) {
	c := compileFixtureSet(t, Deps{})
	if c.Severities.Red != .45 || c.Severities.Risk != .20 || c.Bands.High != .75 {
		t.Errorf("config = severities %+v, bands %+v", c.Severities, c.Bands)
	}
}

func TestCompileFailsOnBadPatternAndNamesRule(t *testing.T) {
	bad := strings.Replace(compileFixture, "match: {preset: payment_no_recourse}", "match: {custom: [\"wire (the ?(deposit\"], regex: true}", 1)
	set, err := Parse([]byte(bad))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err = Compile(set, Deps{}); err == nil || !strings.Contains(err.Error(), "gift_card_payment") {
		t.Errorf("compile error = %v", err)
	}
}

func detectorNames(c Compiled) []string {
	names := make([]string, 0, len(c.Detectors))
	for _, d := range c.Detectors {
		names = append(names, d.Name())
	}
	sort.Strings(names)
	return names
}
func sameStrings(got, want []string) bool {
	return strings.Join(got, "\x00") == strings.Join(want, "\x00")
}
