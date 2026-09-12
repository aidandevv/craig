package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const scamListing = `{"marketplace":"craigslist","title":"Beautiful 2BR - MUST GO TODAY","price":850,"description":"I am currently out of the country. Send the deposit by western union to hold the unit. Reach me at five one zero five five five.","contact":{"relay_only":true}}`
const cleanListing = `{"marketplace":"craigslist","title":"Bright two bedroom near the park","price":2400,"description":"Available from the first of next month. Text or call to arrange a viewing. Deposit is one month's rent, due at lease signing after you see the place.","contact":{"phone":"510-555-1234"}}`

func run(t *testing.T, input string, args ...string) (string, int) {
	t.Helper()
	var out bytes.Buffer
	returnOut := runAnalyze(args, strings.NewReader(input), &out)
	return out.String(), returnOut
}

func decodeAssessment(t *testing.T, raw string) struct {
	RiskScore   float64 `json:"risk_score"`
	RiskBand    string  `json:"risk_band"`
	HardFlagged bool    `json:"hard_flagged"`
	HighRisk    []struct {
		Rule string `json:"rule"`
	} `json:"high_risk"`
	PositiveSignals []struct {
		Rule string `json:"rule"`
	} `json:"positive_signals"`
	NotEvaluated []struct {
		Rule   string `json:"rule"`
		Reason string `json:"reason"`
	} `json:"not_evaluated"`
	Coverage struct {
		Ran     int `json:"ran"`
		Enabled int `json:"enabled"`
	} `json:"coverage"`
} {
	t.Helper()
	var assessment struct {
		RiskScore   float64 `json:"risk_score"`
		RiskBand    string  `json:"risk_band"`
		HardFlagged bool    `json:"hard_flagged"`
		HighRisk    []struct {
			Rule string `json:"rule"`
		} `json:"high_risk"`
		PositiveSignals []struct {
			Rule string `json:"rule"`
		} `json:"positive_signals"`
		NotEvaluated []struct {
			Rule   string `json:"rule"`
			Reason string `json:"reason"`
		} `json:"not_evaluated"`
		Coverage struct {
			Ran     int `json:"ran"`
			Enabled int `json:"enabled"`
		} `json:"coverage"`
	}
	if err := json.Unmarshal([]byte(raw), &assessment); err != nil {
		t.Fatalf("invalid assessment JSON: %v\n%s", err, raw)
	}
	return assessment
}

func TestAnalyzeFlagsScamAndReportsUnavailableImageChecks(t *testing.T) {
	out, code := run(t, "", "--data", scamListing, "--json")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, out)
	}
	got := decodeAssessment(t, out)
	if got.RiskScore < .5 || got.RiskBand != "high" || !got.HardFlagged || len(got.HighRisk) == 0 {
		t.Errorf("scam verdict = %+v", got)
	}
	if len(got.NotEvaluated) != 4 {
		t.Fatalf("not evaluated = %+v", got.NotEvaluated)
	}
	expectedReasons := map[string]string{
		"market_rent_below_hud":     "market_rent_inputs_missing",
		"mls_watermark":             "no_api_key",
		"reverse_image_real_estate": "no_api_key",
		"stock_photos":              "no_api_key",
	}
	for _, entry := range got.NotEvaluated {
		if entry.Reason != expectedReasons[entry.Rule] {
			t.Errorf("%+v", entry)
		}
	}
	if got.Coverage.Ran >= got.Coverage.Enabled {
		t.Errorf("coverage = %+v", got.Coverage)
	}
}

func TestAnalyzeLeavesCleanListingAloneAndShowsDirectContact(t *testing.T) {
	out, code := run(t, "", "--data", cleanListing, "--json")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, out)
	}
	got := decodeAssessment(t, out)
	if got.RiskScore != 0 || got.RiskBand != "low" {
		t.Errorf("clean verdict = %+v", got)
	}
	if len(got.PositiveSignals) != 1 || got.PositiveSignals[0].Rule != "direct_phone_listed" {
		t.Errorf("positive signals = %+v", got.PositiveSignals)
	}
}

func TestAnalyzeGreenSignalCannotLowerScoreEndToEnd(t *testing.T) {
	withGreen := strings.Replace(scamListing, `"contact":{"relay_only":true}`, `"contact":{"relay_only":true,"phone":"510-555-1234"}`, 1)
	base, baseCode := run(t, "", "--data", scamListing, "--json")
	padded, paddedCode := run(t, "", "--data", withGreen, "--json")
	if baseCode != 0 || paddedCode != 0 {
		t.Fatalf("exit codes %d/%d", baseCode, paddedCode)
	}
	if got, want := decodeAssessment(t, padded).RiskScore, decodeAssessment(t, base).RiskScore; got != want {
		t.Errorf("green signal changed score: %v -> %v", want, got)
	}
}

func TestAnalyzeAcceptsFileAndStdinInput(t *testing.T) {
	path := t.TempDir() + "/listing.json"
	if err := writeFile(path, cleanListing); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, input string
		args        []string
	}{{"file", "", []string{"--file", path, "--json"}}, {"stdin", cleanListing, []string{"--file", "-", "--json"}}} {
		t.Run(tc.name, func(t *testing.T) {
			out, code := run(t, tc.input, tc.args...)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, out)
			}
			_ = decodeAssessment(t, out)
		})
	}
}

func TestAnalyzeCustomRuleFileChangesOutputWithoutRebuild(t *testing.T) {
	rules := `version: "1.0"
severities: {red: 0.45, risk: 0.20, green: 0.0}
risk_bands: {caution: 0.30, elevated: 0.55, high: 0.75}
rules:
  custom_marker:
    type: pattern_match
    scope: title
    severity: red
    match: {custom: ["EXCLUSIVE_MARKER"]}
`
	path := t.TempDir() + "/rules.yaml"
	if err := writeFile(path, rules); err != nil {
		t.Fatal(err)
	}
	out, code := run(t, "", "--data", `{"title":"EXCLUSIVE_MARKER"}`, "--rules", path, "--json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	got := decodeAssessment(t, out)
	if got.RiskScore != .45 || len(got.HighRisk) != 1 || got.HighRisk[0].Rule != "custom_marker" {
		t.Errorf("custom rule verdict = %+v", got)
	}
}

func TestAnalyzeTextOutputAndUsageErrors(t *testing.T) {
	out, code := run(t, "", "--data", scamListing)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	for _, want := range []string{"HIGH RISK", "NOT EVALUATED", "checks ran"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	for _, tc := range []struct {
		name string
		args []string
	}{{"no input", []string{"--json"}}, {"both sources", []string{"--data", scamListing, "--file", "x"}}, {"bad JSON", []string{"--data", "{"}}, {"missing file", []string{"--file", "/not-a-real-listing.json"}}, {"extra positional", []string{"--data", scamListing, "extra"}}} {
		t.Run(tc.name, func(t *testing.T) {
			out, code := run(t, "", tc.args...)
			if code != exitUsage || !strings.Contains(out, "error") {
				t.Errorf("got exit %d: %s", code, out)
			}
		})
	}
}

func TestAnalyzeRuleSetErrorsHaveDistinctExitCode(t *testing.T) {
	path := t.TempDir() + "/rules.yaml"
	if err := writeFile(path, "version: \"1.0\"\nrules: {}\n"); err != nil {
		t.Fatal(err)
	}
	out, code := run(t, "", "--data", scamListing, "--rules", path)
	if code != exitRuleSet || !strings.Contains(out, "rule") {
		t.Errorf("got exit %d: %s", code, out)
	}
}
