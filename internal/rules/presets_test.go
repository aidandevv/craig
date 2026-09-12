package rules

import "testing"

func TestMigrateApplicationFeeRuleSplitsTheOldDefaultAndPreservesCustomPolicy(t *testing.T) {
	set, err := DefaultRuleSet()
	if err != nil {
		t.Fatal(err)
	}
	delete(set.Rules, "nonstandard_rental_fee")
	delete(set.Rules, "high_standard_rental_fee")
	changed, err := MigrateApplicationFeeRule(&set)
	if err != nil || !changed {
		t.Fatalf("migration changed=%v err=%v", changed, err)
	}
	if got := set.Rules["nonstandard_rental_fee"]; got.Type != TypeApplicationFeeCheck || got.FeeCheckKind != FeeCheckKindNonstandard {
		t.Errorf("migrated nonstandard rule = %+v", got)
	}
	if got := set.Rules["high_standard_rental_fee"]; got.Type != TypeApplicationFeeCheck || got.FeeCheckKind != FeeCheckKindHighStandard || got.ApplicationFeeHighThreshold != 50 {
		t.Errorf("migrated high standard rule = %+v", got)
	}

	legacyDefault := Rule{Type: TypeApplicationFeeCheck, Severity: SeverityRisk, Scope: ScopeWholePost, Description: "Holding fee or unusually high rental fee (application, admin, processing, credit check, and similar)", ApplicationFeeHighThreshold: 100}
	set.Rules = map[string]Rule{"application_fee_details": legacyDefault}
	changed, err = MigrateApplicationFeeRule(&set)
	if err != nil || !changed || set.Rules["application_fee_details"].Type != "" {
		t.Fatalf("legacy default did not split: changed=%v err=%v rules=%+v", changed, err, set.Rules)
	}

	set.Rules = map[string]Rule{"application_fee_details": {Type: TypeApplicationFeeCheck, Severity: SeverityRisk, Scope: ScopeWholePost, Enabled: boolPtr(false), ApplicationFeeHighThreshold: 250}}
	changed, err = MigrateApplicationFeeRule(&set)
	if err != nil || changed {
		t.Fatalf("existing rule changed=%v err=%v", changed, err)
	}
	if set.Rules["application_fee_details"].ApplicationFeeHighThreshold != 250 || set.Rules["application_fee_details"].IsEnabled() {
		t.Errorf("existing custom fee rule was overwritten: %+v", set.Rules["application_fee_details"])
	}
}

func TestMigrateMarketRentRuleAddsOnlyWhenMissing(t *testing.T) {
	set, err := DefaultRuleSet()
	if err != nil {
		t.Fatal(err)
	}
	delete(set.Rules, "market_rent_below_hud")
	changed, err := MigrateMarketRentRule(&set)
	if err != nil || !changed {
		t.Fatalf("migration changed=%v err=%v", changed, err)
	}
	if got := set.Rules["market_rent_below_hud"]; got.Type != TypeMarketRentCheck || got.Severity != SeverityRisk || got.Hard || got.MarketRentLowRatio != .55 {
		t.Errorf("migrated rule = %+v", got)
	}

	set.Rules["market_rent_below_hud"] = Rule{Type: TypeMarketRentCheck, Severity: SeverityRisk, Enabled: boolPtr(false), MarketRentLowRatio: .4}
	changed, err = MigrateMarketRentRule(&set)
	if err != nil || changed {
		t.Fatalf("existing rule changed=%v err=%v", changed, err)
	}
	if got := set.Rules["market_rent_below_hud"]; got.MarketRentLowRatio != .4 || got.IsEnabled() {
		t.Errorf("existing custom market-rent rule was overwritten: %+v", got)
	}
}

func TestMigrateRentPriceMismatchRuleAddsOnlyWhenMissing(t *testing.T) {
	set, err := DefaultRuleSet()
	if err != nil {
		t.Fatal(err)
	}
	delete(set.Rules, "rent_price_mismatch")
	changed, err := MigrateRentPriceMismatchRule(&set)
	if err != nil || !changed {
		t.Fatalf("migration changed=%v err=%v", changed, err)
	}
	if got := set.Rules["rent_price_mismatch"]; got.Type != TypeRentPriceMismatch || got.Severity != SeverityRisk || got.Hard || got.RentPriceMismatchRatio != .10 {
		t.Errorf("migrated rule = %+v", got)
	}

	set.Rules["rent_price_mismatch"] = Rule{Type: TypeRentPriceMismatch, Severity: SeverityRisk, Enabled: boolPtr(false), RentPriceMismatchRatio: .25}
	changed, err = MigrateRentPriceMismatchRule(&set)
	if err != nil || changed {
		t.Fatalf("existing rule changed=%v err=%v", changed, err)
	}
	if got := set.Rules["rent_price_mismatch"]; got.RentPriceMismatchRatio != .25 || got.IsEnabled() {
		t.Errorf("existing custom price rule was overwritten: %+v", got)
	}
}

func boolPtr(value bool) *bool { return &value }
