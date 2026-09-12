package rules

import (
	"context"
	"strings"
	"testing"

	"github.com/aidandevv/craig-extension/internal/detect"
	"github.com/aidandevv/craig-extension/internal/domain"
)

func mustContactCheck(t *testing.T, name string, r Rule, weight float64) detect.Detector {
	t.Helper()
	d, err := newContactCheck(name, r, weight)
	if err != nil {
		t.Fatalf("compile %s: %v", name, err)
	}
	return d
}

func contactRule(checks ...string) Rule {
	return Rule{
		Type: TypeContactCheck, Scope: ScopeWholePost, Severity: SeverityRisk,
		Checks: checks,
	}
}

func TestContactCheckDetectsObfuscatedDigits(t *testing.T) {
	got, err := mustContactCheck(t, "contact_evasion", contactRule(CheckObfuscatedDigits), 0.20).
		Evaluate(context.Background(), domain.Listing{Description: "reach me at 5 1 0 - 5 5 5 - 1 2 3 4"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got.Risk != 0.20 {
		t.Errorf("got risk %v, want 0.20", got.Risk)
	}
	if len(got.Details) == 0 || !strings.HasPrefix(got.Details[0], "contact_evasion: ") {
		t.Errorf("details %v must be prefixed with the rule name", got.Details)
	}
}

func TestContactCheckIgnoresOrdinaryPhoneNumbers(t *testing.T) {
	for _, text := range []string{
		"call me at 510-555-1234",
		"(510) 555-1234 anytime",
		"unit 12 at 4400 Broadway, built in 1974",
	} {
		got, err := mustContactCheck(t, "contact_evasion", contactRule(CheckObfuscatedDigits), 0.20).
			Evaluate(context.Background(), domain.Listing{Description: text})
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if got.Risk != 0 {
			t.Errorf("%q was flagged as obfuscated; a normal phone number is not evasion", text)
		}
	}
}

func TestContactCheckIgnoresShortNumberedText(t *testing.T) {
	got, err := mustContactCheck(t, "contact_evasion", contactRule(CheckObfuscatedDigits), 0.20).
		Evaluate(context.Background(), domain.Listing{Description: "Call now x 12\nOR Text 12 to show contact info\n2\n1\n2\n3\n4\n5\n6\n7\n8\n9"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got.Risk != 0 || len(got.Flags) != 0 {
		t.Errorf("short numbered UI text was flagged as a hidden phone number: %+v", got)
	}
}

func TestContactCheckIgnoresPlaceholderPhoneSequence(t *testing.T) {
	got, err := mustContactCheck(t, "contact_evasion", contactRule(CheckObfuscatedDigits), 0.20).
		Evaluate(context.Background(), domain.Listing{Description: "2\n1\n2\n3\n4\n5\n6\n7\n8\n9"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got.Risk != 0 || len(got.Flags) != 0 {
		t.Errorf("placeholder phone sequence was flagged: %+v", got)
	}
}

func TestContactCheckDetectsSpelledOutDigits(t *testing.T) {
	got, err := mustContactCheck(t, "contact_evasion", contactRule(CheckSpelledOutDigits), 0.20).
		Evaluate(context.Background(), domain.Listing{Description: "five one zero five five five"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got.Risk != 0.20 {
		t.Errorf("got risk %v, want 0.20", got.Risk)
	}
}

func TestContactCheckIgnoresOrdinaryNumberWords(t *testing.T) {
	got, err := mustContactCheck(t, "contact_evasion", contactRule(CheckSpelledOutDigits), 0.20).
		Evaluate(context.Background(), domain.Listing{Description: "two bedroom, one bath, available in one week"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got.Risk != 0 {
		t.Error("ordinary prose containing number words was flagged")
	}
}

func TestContactCheckDetectsEmailObfuscation(t *testing.T) {
	for _, text := range []string{
		"write to jane at gmail dot com",
		"jane (at) gmail (dot) com",
		"jane [at] gmail [dot] com",
	} {
		got, err := mustContactCheck(t, "contact_evasion", contactRule(CheckEmailObfuscation), 0.20).
			Evaluate(context.Background(), domain.Listing{Description: text})
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if got.Risk != 0.20 {
			t.Errorf("%q was not detected as an obfuscated email", text)
		}
	}
}

func TestContactCheckReadsPageAffordances(t *testing.T) {
	relay, err := mustContactCheck(t, "relay", contactRule(CheckRelayOnly), 0.20).
		Evaluate(context.Background(), domain.Listing{Contact: domain.Contact{RelayOnly: true}})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if relay.Risk != 0.20 {
		t.Errorf("relay_only affordance not detected, got risk %v", relay.Risk)
	}

	app, err := mustContactCheck(t, "app", contactRule(CheckAppOnly), 0.20).
		Evaluate(context.Background(), domain.Listing{Contact: domain.Contact{AppOnly: true}})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if app.Risk != 0.20 {
		t.Errorf("app_only affordance not detected, got risk %v", app.Risk)
	}
}

func TestContactCheckGreenRuleMatchesButScoresZero(t *testing.T) {
	rule := contactRule(CheckDirectPhonePresent)
	rule.Severity = SeverityGreen
	got, err := mustContactCheck(t, "direct_phone_listed", rule, 0).
		Evaluate(context.Background(), domain.Listing{Contact: domain.Contact{Phone: "510-555-1234"}})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(got.Flags) != 1 || got.Flags[0] != "direct_phone_listed" {
		t.Errorf("got flags %v, want the green rule to report a match", got.Flags)
	}
	if got.Risk != 0 {
		t.Errorf("a green rule must contribute no risk, got %v", got.Risk)
	}
}

func TestContactCheckRequireAll(t *testing.T) {
	rule := contactRule(CheckRelayOnly, CheckAppOnly)
	rule.RequireAll = true
	detector := mustContactCheck(t, "both", rule, 0.20)

	partial, err := detector.Evaluate(context.Background(),
		domain.Listing{Contact: domain.Contact{RelayOnly: true}})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if partial.Risk != 0 {
		t.Errorf("require_all matched on one of two checks, got risk %v", partial.Risk)
	}

	full, err := detector.Evaluate(context.Background(),
		domain.Listing{Contact: domain.Contact{RelayOnly: true, AppOnly: true}})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if full.Risk != 0.20 {
		t.Errorf("require_all did not match with both checks true, got risk %v", full.Risk)
	}
}

func TestContactCheckDefaultsToAnyCheckMatching(t *testing.T) {
	got, err := mustContactCheck(t, "any", contactRule(CheckRelayOnly, CheckAppOnly), 0.20).
		Evaluate(context.Background(), domain.Listing{Contact: domain.Contact{AppOnly: true}})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got.Risk != 0.20 {
		t.Errorf("default require_all=false must match on any check, got risk %v", got.Risk)
	}
}

func TestContactCheckMissingTextIsNotEvaluated(t *testing.T) {
	got, err := mustContactCheck(t, "contact_evasion", contactRule(CheckEmailObfuscation), 0.20).
		Evaluate(context.Background(), domain.Listing{})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got.Skipped != domain.SkipMissingField {
		t.Errorf("got skip %q, want %q", got.Skipped, domain.SkipMissingField)
	}
}

func TestContactCheckRejectsUnknownCheckAtCompileTime(t *testing.T) {
	_, err := newContactCheck("bad", contactRule("telepathy"), 0.20)
	if err == nil {
		t.Fatal("expected a compile error for an unknown check")
	}
	if !strings.Contains(err.Error(), "telepathy") {
		t.Errorf("error %q must name the unknown check", err)
	}
}

func TestParserAcceptsEveryKnownCheck(t *testing.T) {
	for _, check := range allChecks() {
		if !knownCheck(check) {
			t.Errorf("knownCheck rejects %q, which newContactCheck implements", check)
		}
	}
}
