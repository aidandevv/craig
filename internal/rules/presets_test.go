package rules

import "testing"

func TestMigrateApplicationFeeRuleAddsOnlyWhenMissing(t *testing.T) {
	set, err := DefaultRuleSet()
	if err != nil {
		t.Fatal(err)
	}
	delete(set.Rules, "application_fee_details")
	changed, err := MigrateApplicationFeeRule(&set)
	if err != nil || !changed {
		t.Fatalf("migration changed=%v err=%v", changed, err)
	}
	if got := set.Rules["application_fee_details"]; got.Type != TypeApplicationFeeCheck || got.Severity != SeverityRisk {
		t.Errorf("migrated rule = %+v", got)
	}

	set.Rules["application_fee_details"] = Rule{Type: TypeApplicationFeeCheck, Severity: SeverityRisk, Scope: ScopeWholePost, Enabled: boolPtr(false), ApplicationFeeHighThreshold: 250}
	changed, err = MigrateApplicationFeeRule(&set)
	if err != nil || changed {
		t.Fatalf("existing rule changed=%v err=%v", changed, err)
	}
	if set.Rules["application_fee_details"].ApplicationFeeHighThreshold != 250 || set.Rules["application_fee_details"].IsEnabled() {
		t.Errorf("existing custom fee rule was overwritten: %+v", set.Rules["application_fee_details"])
	}
}

func boolPtr(value bool) *bool { return &value }
