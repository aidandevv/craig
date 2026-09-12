package risk

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aidandevv/craig-extension/internal/domain"
	"github.com/aidandevv/craig-extension/internal/rules"
)

func sameStrings(got, want []string) bool {
	return strings.Join(got, "\x00") == strings.Join(want, "\x00")
}

func assessmentFixture() rules.Compiled {
	weight := .28
	return rules.Compiled{
		Coverage:    map[string][]string{"gift_card_payment": {"gift_card_payment"}, "my_landlord_tell": {"my_landlord_tell"}, "contact_evasion": {"contact_evasion"}, "direct_phone_listed": {"direct_phone_listed"}},
		Unavailable: map[string]string{"reverse_image_real_estate": domain.SkipBudgetExhausted, "stock_photos": domain.SkipBudgetExhausted, "mls_watermark": domain.SkipBudgetExhausted},
		Rules: map[string]rules.Rule{
			"gift_card_payment": {Severity: rules.SeverityRed, Description: "Gift-card payment requested"}, "my_landlord_tell": {Severity: rules.SeverityRisk, Weight: &weight, Description: "Keys to be shipped"},
			"contact_evasion": {Severity: rules.SeverityRisk}, "direct_phone_listed": {Severity: rules.SeverityGreen}, "reverse_image_real_estate": {Severity: rules.SeverityRed}, "stock_photos": {Severity: rules.SeverityRisk}, "mls_watermark": {Severity: rules.SeverityRed},
		}, Severities: rules.Severities{Red: .45, Risk: .20}, Bands: testBands,
	}
}
func assessmentResults() []domain.SignalResult {
	return []domain.SignalResult{
		{Name: "gift_card_payment", Risk: .45, Flags: []string{"gift_card_payment"}, Details: []string{`gift_card_payment: matched "gift cards"`}},
		{Name: "my_landlord_tell", Risk: .28, Flags: []string{"my_landlord_tell"}, Details: []string{`my_landlord_tell: matched "keys will be shipped"`}},
		{Name: "contact_evasion"}, {Name: "direct_phone_listed", Flags: []string{"direct_phone_listed"}, Details: []string{"direct_phone_listed: direct phone"}},
	}
}

func TestAssessWorkedExampleBucketsAndCoverage(t *testing.T) {
	got := Assess(assessmentResults(), assessmentFixture(), 42*time.Millisecond)
	if got.RiskScore != .73 || got.RiskBand != BandElevated || got.HardFlagged {
		t.Errorf("verdict = %+v", got)
	}
	if got.Coverage != (Coverage{Ran: 4, Enabled: 7}) || got.AnalysisTimeMS != 42 {
		t.Errorf("coverage/timing = %+v/%d", got.Coverage, got.AnalysisTimeMS)
	}
	if len(got.HighRisk) != 1 || got.HighRisk[0].Rule != "gift_card_payment" || got.HighRisk[0].Label != "Gift-card payment requested" || got.HighRisk[0].Detail != `matched "gift cards"` {
		t.Errorf("high risk = %+v", got.HighRisk)
	}
	if len(got.PotentiallyRisky) != 1 || got.PotentiallyRisky[0].Rule != "my_landlord_tell" || len(got.PositiveSignals) != 1 || got.PositiveSignals[0].Weight != 0 || !sameStrings(got.PassedChecks, []string{"contact_evasion"}) {
		t.Errorf("buckets = %+v", got)
	}
}

func TestAssessUnavailableRulesAreNotEvaluated(t *testing.T) {
	got := Assess(assessmentResults(), assessmentFixture(), 0)
	if len(got.NotEvaluated) != 3 {
		t.Fatalf("not evaluated = %+v", got.NotEvaluated)
	}
	for _, finding := range got.NotEvaluated {
		if finding.Reason != domain.SkipBudgetExhausted {
			t.Errorf("%+v", finding)
		}
	}
}

func TestAssessHardAndSharedDetectorBehavior(t *testing.T) {
	c := assessmentFixture()
	c.Coverage["reverse_image"] = []string{"reverse_image_real_estate", "stock_photos"}
	delete(c.Unavailable, "reverse_image_real_estate")
	delete(c.Unavailable, "stock_photos")
	got := Assess([]domain.SignalResult{{Name: "reverse_image", Risk: .60, Hard: true, Flags: []string{"reverse_image_real_estate"}, Details: []string{`reverse_image_real_estate: matched "zillow"`}, ImageMatches: []domain.ImageMatchEvidence{{Rule: "reverse_image_real_estate", ListingImageURL: "https://images.example/listing.jpg", SourcePageURL: "https://zillow.example/home", SourceImageURL: "https://zillow.example/photo.jpg"}}}}, c, 0)
	if got.RiskBand != BandHigh || !got.HardFlagged || len(got.HighRisk) != 1 || got.HighRisk[0].Rule != "reverse_image_real_estate" || !sameStrings(got.PassedChecks, []string{"stock_photos"}) {
		t.Errorf("assessment = %+v", got)
	}
	if matches := got.HighRisk[0].ImageMatches; len(matches) != 1 || matches[0].SourcePageURL != "https://zillow.example/home" {
		t.Errorf("image evidence = %+v", matches)
	}
}

func TestAssessExpandsSkippedSharedDetector(t *testing.T) {
	c := assessmentFixture()
	c.Coverage["reverse_image"] = []string{"reverse_image_real_estate", "stock_photos"}
	delete(c.Unavailable, "reverse_image_real_estate")
	delete(c.Unavailable, "stock_photos")
	got := Assess([]domain.SignalResult{{Name: "reverse_image", Skipped: domain.SkipNoImages}}, c, 0)
	var rules []string
	for _, entry := range got.NotEvaluated {
		if entry.Reason == domain.SkipNoImages {
			rules = append(rules, entry.Rule)
		}
	}
	sort.Strings(rules)
	if !sameStrings(rules, []string{"reverse_image_real_estate", "stock_photos"}) {
		t.Errorf("expanded skips = %v", rules)
	}
}

func TestEveryEnabledRuleLandsInExactlyOneBucket(t *testing.T) {
	c := assessmentFixture()
	got := Assess(assessmentResults(), c, 0)
	seen := map[string]int{}
	for _, group := range [][]Finding{got.HighRisk, got.PotentiallyRisky, got.PositiveSignals} {
		for _, f := range group {
			seen[f.Rule]++
		}
	}
	for _, name := range got.PassedChecks {
		seen[name]++
	}
	for _, f := range got.NotEvaluated {
		seen[f.Rule]++
	}
	for name := range c.Rules {
		if seen[name] != 1 {
			t.Errorf("%s appears %d times", name, seen[name])
		}
	}
}

func TestAssessOutputIsDeterministic(t *testing.T) {
	first := Assess(assessmentResults(), assessmentFixture(), 0)
	for i := 0; i < 20; i++ {
		got := Assess(assessmentResults(), assessmentFixture(), 0)
		if !sameStrings(got.PassedChecks, first.PassedChecks) || len(got.NotEvaluated) != len(first.NotEvaluated) {
			t.Fatal("output ordering varied")
		}
		for j := range got.NotEvaluated {
			if got.NotEvaluated[j] != first.NotEvaluated[j] {
				t.Fatal("not-evaluated ordering varied")
			}
		}
	}
}

func TestIncompletePhotosDoNotPassUnmatchedRules(t *testing.T) {
	compiled := assessmentFixture()
	compiled.Coverage["reverse_image"] = []string{"reverse_image_real_estate", "stock_photos"}
	got := Assess([]domain.SignalResult{{Name: "reverse_image", ImagesChecked: 1, ImagesTotal: 2, Flags: []string{"reverse_image_real_estate"}, Risk: .6, Hard: true}}, compiled, 0)
	if len(got.HighRisk) != 1 || len(got.PassedChecks) != 0 {
		t.Fatalf("lost match or passed incomplete check: %+v", got)
	}
	found := false
	for _, rule := range got.NotEvaluated {
		if rule.Rule == "stock_photos" && rule.Reason == domain.SkipPartialImages {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing incomplete stock check: %+v", got)
	}
	if len(got.ImageCoverage) != 1 || got.ImageCoverage[0].Checked != 1 {
		t.Fatalf("missing image coverage: %+v", got)
	}
}
