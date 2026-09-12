package rules

import (
	"context"
	"strings"
	"testing"

	"github.com/aidandevv/craig-extension/internal/domain"
)

func mismatchRule() Rule {
	return Rule{Type: TypeRentPriceMismatch, Severity: SeverityRisk, RentPriceMismatchRatio: .10}
}

func TestRentPriceMismatchFlagsMateriallyDifferentAdvertisedRent(t *testing.T) {
	detector, err := newRentPriceMismatch("rent_price_mismatch", mismatchRule(), .15)
	if err != nil {
		t.Fatal(err)
	}
	result, err := detector.Evaluate(context.Background(), domain.Listing{Price: 2100, Title: "$2,100 / 1br - $1,795 New Plank Floor"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Risk != .15 || len(result.Flags) != 1 || !strings.Contains(result.Details[0], "$1795") || !strings.Contains(result.Details[0], "15%") {
		t.Errorf("result = %+v", result)
	}
}

func TestRentPriceMismatchIgnoresSameOrSmalllyDifferentRent(t *testing.T) {
	detector, err := newRentPriceMismatch("rent_price_mismatch", mismatchRule(), .15)
	if err != nil {
		t.Fatal(err)
	}
	for _, listing := range []domain.Listing{
		{Price: 2100, Title: "$2,100 / 1br"},
		{Price: 2100, Title: "$2,050 / 1br"},
		{Price: 2100, Title: "One bedroom"},
	} {
		result, err := detector.Evaluate(context.Background(), listing)
		if err != nil {
			t.Fatal(err)
		}
		if result.Risk != 0 || len(result.Flags) != 0 {
			t.Errorf("listing %+v got unexpected mismatch %+v", listing, result)
		}
	}
}

func TestRentPriceMismatchReportsMissingInputs(t *testing.T) {
	detector, err := newRentPriceMismatch("rent_price_mismatch", mismatchRule(), .15)
	if err != nil {
		t.Fatal(err)
	}
	result, err := detector.Evaluate(context.Background(), domain.Listing{Title: "$1,800 studio"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Skipped != domain.SkipRentPriceInputsMissing {
		t.Errorf("result = %+v", result)
	}
}
