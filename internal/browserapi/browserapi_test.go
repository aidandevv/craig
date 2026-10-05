package browserapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aidandevv/craig/internal/rules"
	"strings"
	"testing"
	"time"
)

const scamRequest = `{
  "listing": {
    "marketplace": "craigslist",
    "listing_url": "https://sfbay.craigslist.org/apa/1.html",
    "title": "Beautiful 2BR - MUST GO TODAY",
    "description": "I am out of the country. Send the deposit by western union to hold the unit.",
    "contact": {"relay_only": true}
  },
  "vision": {"monthly_cap": 999, "max_images": 4}
}`

func TestAnalyzeWithoutKeyScoresOfflineAndTraces(t *testing.T) {
	out, err := Host{}.Analyze(context.Background(), []byte(scamRequest))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		HardFlagged  bool `json:"hard_flagged"`
		NotEvaluated []struct {
			Reason string `json:"reason"`
		} `json:"not_evaluated"`
		Trace []struct {
			Message string `json:"message"`
		} `json:"trace"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if !got.HardFlagged || len(got.Trace) == 0 {
		t.Errorf("want hard flag and trace, got %s", out)
	}
	for _, event := range got.Trace {
		if strings.Contains(strings.ToLower(event.Message), "western union") {
			t.Errorf("trace leaked listing text: %q", event.Message)
		}
	}
}

func TestAnalyzeRejectsUnknownFields(t *testing.T) {
	_, err := Host{}.Analyze(context.Background(), []byte(`{"listing":{},"surprise":1}`))
	if err == nil {
		t.Fatal("unknown top-level field must be rejected")
	}
}

func TestPrepareRulesRoundTripsDefaults(t *testing.T) {
	defaults, err := DefaultRules()
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareRules(defaults)
	if err != nil {
		t.Fatal(err)
	}
	var a, b map[string]any
	_ = json.Unmarshal(defaults, &a)
	_ = json.Unmarshal(prepared, &b)
	if len(a["rules"].(map[string]any)) != len(b["rules"].(map[string]any)) {
		t.Errorf("rule count changed: %d → %d", len(a["rules"].(map[string]any)), len(b["rules"].(map[string]any)))
	}
}

func TestPrepareRulesRejectsEmptySet(t *testing.T) {
	if _, err := PrepareRules([]byte(`{}`)); err == nil {
		t.Fatal("empty rule set must be rejected")
	}
}

func TestRuleSchemaListsPresets(t *testing.T) {
	out, err := RuleSchema()
	if err != nil || !strings.Contains(string(out), `"preset_names"`) {
		t.Fatalf("schema = %s, err = %v", out, err)
	}
}

func TestDecodeEvidence(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	encoded, err := EncodeEvidence(map[string]any{"responses": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	fresh := `{"response":` + mustQuote(encoded) + `,"checked_at":"2026-09-27T01:00:00Z"}`
	stale := `{"response":` + mustQuote(encoded) + `,"checked_at":"2026-09-26T11:59:59Z"}`
	cases := map[string]bool{fresh: true, stale: false, "not json": false, `{"response":"not json","checked_at":"2026-09-27T01:00:00Z"}`: false}
	for raw, want := range cases {
		if _, _, ok := DecodeEvidence(raw, now); ok != want {
			t.Errorf("DecodeEvidence(%q) ok = %v, want %v", raw, ok, want)
		}
	}
}

func mustQuote(s string) string { b, _ := json.Marshal(s); return string(b) }

func TestRequestResourceLimits(t *testing.T) {
	if _, err := PrepareRules([]byte(strings.Repeat(" ", MaxRequestBytes+1))); err == nil {
		t.Fatal("oversized request accepted")
	}
	for _, max := range []int{0, -1, 25} {
		raw := strings.Replace(scamRequest, `"max_images": 4`, `"max_images": `+fmt.Sprint(max), 1)
		if _, err := (Host{}).Analyze(context.Background(), []byte(raw)); err == nil {
			t.Errorf("max_images %d accepted", max)
		}
	}
	defaults, err := DefaultRules()
	if err != nil {
		t.Fatal(err)
	}
	var set rules.RuleSet
	if err := json.Unmarshal(defaults, &set); err != nil {
		t.Fatal(err)
	}
	base := set.Rules["urgency_pressure"]
	for i := 0; i < 257; i++ {
		set.Rules[fmt.Sprintf("large_%d", i)] = base
	}
	encoded, _ := json.Marshal(set)
	if _, err := PrepareRules(encoded); err == nil {
		t.Fatal("oversized rule set accepted")
	}
}
