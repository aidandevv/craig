package rules

import (
	"context"
	"strings"
	"testing"

	"github.com/aidandevv/craig-extension/internal/domain"
)

func feeRule() Rule {
	return Rule{Type: TypeApplicationFeeCheck, Severity: SeverityRisk, Scope: ScopeWholePost, ApplicationFeeHighThreshold: 100}
}

func nonstandardFeeRule() Rule {
	return Rule{Type: TypeApplicationFeeCheck, Severity: SeverityRisk, Scope: ScopeWholePost, FeeCheckKind: FeeCheckKindNonstandard}
}

func highStandardFeeRule() Rule {
	return Rule{Type: TypeApplicationFeeCheck, Severity: SeverityRisk, Scope: ScopeWholePost, FeeCheckKind: FeeCheckKindHighStandard, ApplicationFeeHighThreshold: 50}
}

func TestApplicationFeeCheckSeparatesNonstandardAndHighStandardFees(t *testing.T) {
	nonstandard, err := newApplicationFeeCheck("nonstandard_rental_fee", nonstandardFeeRule(), .20)
	if err != nil {
		t.Fatal(err)
	}
	high, err := newApplicationFeeCheck("high_standard_rental_fee", highStandardFeeRule(), .20)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		text             string
		wantNonstandard  bool
		wantHighStandard bool
	}{
		{"A $25 holding fee is due before the tour.", true, false},
		{"The application fee is $55 per adult.", false, true},
		{"The application fee is $50.", false, false},
	} {
		nonstandardResult, err := nonstandard.Evaluate(context.Background(), domain.Listing{Description: tc.text})
		if err != nil {
			t.Fatal(err)
		}
		highResult, err := high.Evaluate(context.Background(), domain.Listing{Description: tc.text})
		if err != nil {
			t.Fatal(err)
		}
		if got := len(nonstandardResult.Flags) == 1; got != tc.wantNonstandard {
			t.Errorf("%q nonstandard match = %v, want %v (%+v)", tc.text, got, tc.wantNonstandard, nonstandardResult)
		}
		if got := len(highResult.Flags) == 1; got != tc.wantHighStandard {
			t.Errorf("%q high-standard match = %v, want %v (%+v)", tc.text, got, tc.wantHighStandard, highResult)
		}
	}
}

func TestApplicationFeeCheckRecognizesAdditionalNonstandardFees(t *testing.T) {
	detector, err := newApplicationFeeCheck("nonstandard_rental_fee", nonstandardFeeRule(), .20)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		text string
		want string
	}{
		{"Pay $1 key money to secure the apartment.", "key money"},
		{"A refundable option fee is required before a lease is offered.", "option fee"},
		{"The $10 commitment fee reserves your place in line.", "commitment fee"},
		{"A priority fee applies to all applicants.", "priority fee"},
		{"Pay a waitlist fee before you can tour.", "waitlist fee"},
		{"An application deposit is due before approval.", "application deposit"},
		{"A good-faith deposit is required to continue.", "good-faith deposit"},
		{"The pre-lease fee is due today.", "pre-lease fee"},
	} {
		result, err := detector.Evaluate(context.Background(), domain.Listing{Description: tc.text})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Flags) != 1 || !strings.Contains(result.Details[0], tc.want) {
			t.Errorf("%q result = %+v, want %q", tc.text, result, tc.want)
		}
	}
}

func TestApplicationFeeCheckFlagsHoldingAndHighApplicationFeesAsRisk(t *testing.T) {
	detector, err := newApplicationFeeCheck("application_fee_details", feeRule(), .20)
	if err != nil {
		t.Fatal(err)
	}
	result, err := detector.Evaluate(context.Background(), domain.Listing{
		Description: "A $125 application fee and a holding fee are due before we schedule a tour.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Hard || result.Risk != .20 || len(result.Flags) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Details) != 1 || !strings.Contains(result.Details[0], "$125") || !strings.Contains(result.Details[0], "holding fee") {
		t.Errorf("detail = %v", result.Details)
	}
}

func TestApplicationFeeCheckLeavesOrdinaryFeeAlone(t *testing.T) {
	detector, err := newApplicationFeeCheck("application_fee_details", feeRule(), .20)
	if err != nil {
		t.Fatal(err)
	}
	result, err := detector.Evaluate(context.Background(), domain.Listing{Description: "The application fee is $45."})
	if err != nil {
		t.Fatal(err)
	}
	if result.Risk != 0 || len(result.Flags) != 0 || result.Skipped != "" {
		t.Errorf("ordinary fee result = %+v", result)
	}
}

func TestApplicationFeeCheckRecognizesAdminHoldingFeeFormatting(t *testing.T) {
	detector, err := newApplicationFeeCheck("application_fee_details", feeRule(), .20)
	if err != nil {
		t.Fatal(err)
	}
	result, err := detector.Evaluate(context.Background(), domain.Listing{
		Description: "Refundable://$100 admin holding fee and +/-$29.95 application fee.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Flags) != 1 || result.Risk != .20 || !strings.Contains(result.Details[0], "holding fee") {
		t.Errorf("screenshot-shaped fee result = %+v", result)
	}
}

func TestApplicationFeeCheckFlagsNamedFeesAboveTheirNormalCeiling(t *testing.T) {
	detector, err := newApplicationFeeCheck("application_fee_details", feeRule(), .20)
	if err != nil {
		t.Fatal(err)
	}
	result, err := detector.Evaluate(context.Background(), domain.Listing{
		Description: "The credit check fee is $85 per applicant.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Hard || result.Risk != .20 || len(result.Flags) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if !strings.Contains(result.Details[0], "$85") || !strings.Contains(result.Details[0], "credit or background check fee") {
		t.Errorf("detail = %v", result.Details)
	}
}

func TestApplicationFeeCheckRecognizesCommonFeeSynonyms(t *testing.T) {
	detector, err := newApplicationFeeCheck("application_fee_details", feeRule(), .20)
	if err != nil {
		t.Fatal(err)
	}
	result, err := detector.Evaluate(context.Background(), domain.Listing{
		Description: "Application processing charge: $125. A $85 tenant screening cost and a lease setup fee of $950 are due.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Risk != .20 || len(result.Flags) != 1 {
		t.Fatalf("result = %+v", result)
	}
	for _, want := range []string{"application fee", "tenant screening fee", "lease initiation fee"} {
		if !strings.Contains(result.Details[0], want) {
			t.Errorf("detail %q does not explain %s", result.Details[0], want)
		}
	}
}

func TestApplicationFeeCheckLeavesNamedFeesWithinNormalRangeAlone(t *testing.T) {
	detector, err := newApplicationFeeCheck("application_fee_details", feeRule(), .20)
	if err != nil {
		t.Fatal(err)
	}
	result, err := detector.Evaluate(context.Background(), domain.Listing{
		Description: "A refundable $250 pet fee applies. Processing fee is $40.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Risk != 0 || len(result.Flags) != 0 {
		t.Errorf("result = %+v", result)
	}
}

func TestApplicationFeeCheckDescribesFeesFarAboveNormalButNeverGoesHard(t *testing.T) {
	detector, err := newApplicationFeeCheck("application_fee_details", feeRule(), .20)
	if err != nil {
		t.Fatal(err)
	}
	result, err := detector.Evaluate(context.Background(), domain.Listing{
		Description: "A non-refundable $350 application fee and a $600 move-in fee are due at signing.",
	})
	if err != nil {
		t.Fatal(err)
	}
	// application_fee_check is parser-enforced as risk severity and never hard
	// (see parser.go): a fee amount alone, however extreme, is never treated as
	// proof of wrongdoing. Only its detail text should read more urgently.
	if result.Hard {
		t.Fatalf("a fee amount alone must never set Hard, got %+v", result)
	}
	if !strings.Contains(result.Details[0], "far above the typical") {
		t.Errorf("detail = %v", result.Details)
	}
}

func TestApplicationFeeCheckNeedsListingText(t *testing.T) {
	detector, err := newApplicationFeeCheck("application_fee_details", feeRule(), .20)
	if err != nil {
		t.Fatal(err)
	}
	result, err := detector.Evaluate(context.Background(), domain.Listing{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Skipped != domain.SkipMissingField {
		t.Errorf("skipped = %q", result.Skipped)
	}
}
