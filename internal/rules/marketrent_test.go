package rules

import (
	"context"
	"strings"
	"testing"

	"github.com/aidandevv/craig-extension/internal/domain"
)

func marketRentRule() Rule {
	return Rule{Type: TypeMarketRentCheck, Severity: SeverityRisk, MarketRentLowRatio: .55}
}

func TestMarketRentCheckFlagsOnlyAnExtremeBelowBenchmarkRent(t *testing.T) {
	detector, err := newMarketRentCheck("market_rent_below_hud", marketRentRule(), .15)
	if err != nil {
		t.Fatal(err)
	}
	bedrooms := 1
	result, err := detector.Evaluate(context.Background(), domain.Listing{
		Price: 1754, Currency: "USD", RentPeriod: "monthly", Bedrooms: &bedrooms, ZIPCode: "94103",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Hard || result.Risk != .15 || len(result.Flags) != 1 || result.Flags[0] != "market_rent_below_hud" {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Details) != 1 || !strings.Contains(result.Details[0], "$1754") || !strings.Contains(result.Details[0], "HUD FY 2027") || !strings.Contains(result.Details[0], "not proof of fraud") {
		t.Errorf("detail = %v", result.Details)
	}
}

func TestMarketRentCheckDoesNotFlagRentAboveConfiguredRatio(t *testing.T) {
	detector, err := newMarketRentCheck("market_rent_below_hud", marketRentRule(), .15)
	if err != nil {
		t.Fatal(err)
	}
	bedrooms := 1
	result, err := detector.Evaluate(context.Background(), domain.Listing{
		Price: 1755, Currency: "USD", RentPeriod: "monthly", Bedrooms: &bedrooms, ZIPCode: "94103",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Risk != 0 || result.Hard || len(result.Flags) != 0 || result.Skipped != "" {
		t.Errorf("result = %+v", result)
	}
}

func TestMarketRentCheckReportsMissingInputsAndUnsupportedZIPInsteadOfGuessing(t *testing.T) {
	detector, err := newMarketRentCheck("market_rent_below_hud", marketRentRule(), .15)
	if err != nil {
		t.Fatal(err)
	}
	missing, err := detector.Evaluate(context.Background(), domain.Listing{})
	if err != nil {
		t.Fatal(err)
	}
	if missing.Skipped != domain.SkipMarketRentInputsMissing {
		t.Errorf("missing-input result = %+v", missing)
	}

	bedrooms := 1
	unsupported, err := detector.Evaluate(context.Background(), domain.Listing{
		Price: 600, Currency: "USD", RentPeriod: "monthly", Bedrooms: &bedrooms, ZIPCode: "99999",
	})
	if err != nil {
		t.Fatal(err)
	}
	if unsupported.Skipped != domain.SkipMarketRentBenchmarkUnavailable || unsupported.Risk != 0 {
		t.Errorf("unsupported-ZIP result = %+v", unsupported)
	}
}
