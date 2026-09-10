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
