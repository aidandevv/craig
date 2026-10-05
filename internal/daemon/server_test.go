package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/aidandevv/craig/internal/cache"
	"github.com/aidandevv/craig/internal/config"
	"github.com/aidandevv/craig/internal/rules"
	"github.com/aidandevv/craig/internal/signals"
)

const testToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func newTestServer(t *testing.T, autoReload bool) (*Server, string) {
	t.Helper()
	rulesPath := filepath.Join(t.TempDir(), "rules.yaml")
	if err := config.WriteFileSecure(rulesPath, rules.DefaultRulesYAML()); err != nil {
		t.Fatalf("write rules: %v", err)
	}
	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	vision := signals.NewVision(store, signals.NopMeter{}, signals.Options{MonthlyCap: 10})
	server, err := New(Options{
		Port: 8765, Token: testToken, DaemonID: "0123456789abcdef0123456789abcdef", RulesPath: rulesPath, AutoReload: autoReload,
		CacheLocation: "cache.db", Vision: vision,
		SetVisionAPIKey: func(apiKey string) (*signals.Vision, error) {
			return signals.NewVision(store, signals.NopMeter{}, signals.Options{APIKey: apiKey, MonthlyCap: 10}), nil
		},
	})
	if err != nil {
		t.Fatalf("new daemon: %v", err)
	}
	return server, rulesPath
}

func TestNewMigratesAnOlderRulesFileWithNewCautionRules(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "rules.yaml")
	legacy := `version: "1.0"
severities: {red: 0.45, risk: 0.20, green: 0.0}
risk_bands: {caution: 0.30, elevated: 0.55, high: 0.75}
rules:
  legacy_marker:
    type: pattern_match
    scope: whole_post
    severity: risk
    match: {custom: ["legacy marker"]}
`
	if err := config.WriteFileSecure(rulesPath, []byte(legacy)); err != nil {
		t.Fatal(err)
	}
	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	server, err := New(Options{Port: 8765, Token: testToken, RulesPath: rulesPath, Vision: signals.NewVision(store, nil, signals.Options{MonthlyCap: 10})})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := server.eng.Compiled().Rules["nonstandard_rental_fee"]; !ok {
		t.Fatal("migrated nonstandard-fee rule is not active")
	}
	if _, ok := server.eng.Compiled().Rules["high_standard_rental_fee"]; !ok {
		t.Fatal("migrated high-standard-fee rule is not active")
	}
	if _, ok := server.eng.Compiled().Rules["market_rent_below_hud"]; !ok {
		t.Fatal("migrated market-rent rule is not active")
	}
	if _, ok := server.eng.Compiled().Rules["rent_price_mismatch"]; !ok {
		t.Fatal("migrated price-mismatch rule is not active")
	}
	if _, ok := server.eng.Compiled().Rules["prepayment_before_access"]; !ok {
		t.Fatal("migrated prepayment-before-access rule is not active")
	}
	raw, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("nonstandard_rental_fee")) || !bytes.Contains(raw, []byte("high_standard_rental_fee")) || !bytes.Contains(raw, []byte("market_rent_below_hud")) || !bytes.Contains(raw, []byte("rent_price_mismatch")) || !bytes.Contains(raw, []byte("prepayment_before_access")) {
		t.Fatalf("migrated rule was not persisted: %s", raw)
	}
	if score := analysisScore(t, server, "A $125 application fee and admin holding fee."); score != .40 {
		t.Errorf("fee migration score = %v, want .40", score)
	}
}

func TestAnalyzeAddsOnlyCautionForExtremeBelowHUDMarketRent(t *testing.T) {
	server, _ := newTestServer(t, false)
	payload := `{"marketplace":"craigslist","listing_url":"https://sfbay.craigslist.org/apa/1.html","title":"One bedroom","price":1600,"currency":"USD","rent_period":"monthly","bedrooms":1,"zip_code":"94103"}`
	response := serve(server, authorized(http.MethodPost, "/api/analyze", payload))
	if response.Code != http.StatusOK {
		t.Fatalf("analyze status = %d: %s", response.Code, response.Body.String())
	}
	var assessment struct {
		RiskScore        float64 `json:"risk_score"`
		HardFlagged      bool    `json:"hard_flagged"`
		PotentiallyRisky []struct {
			Rule   string  `json:"rule"`
			Detail string  `json:"detail"`
			Weight float64 `json:"weight"`
		} `json:"potentially_risky"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &assessment); err != nil {
		t.Fatal(err)
	}
	if assessment.RiskScore != .15 || assessment.HardFlagged || len(assessment.PotentiallyRisky) != 1 {
		t.Fatalf("assessment = %+v", assessment)
	}
	finding := assessment.PotentiallyRisky[0]
	if finding.Rule != "market_rent_below_hud" || finding.Weight != .15 || !strings.Contains(finding.Detail, "not proof of fraud") {
		t.Errorf("market-rent finding = %+v", finding)
	}
}

func request(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host = "127.0.0.1:8765"
	return r
}

func authorized(method, path, body string) *http.Request {
	r := request(method, path, body)
	r.Header.Set("Authorization", "Bearer "+testToken)
	return r
}

func serve(server *Server, request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	return recorder
}

func TestRequestGuardRejectsUnauthenticatedBrowserAndRebindingRequests(t *testing.T) {
	server, _ := newTestServer(t, false)
	cases := []struct {
		name string
		req  *http.Request
		want int
	}{
		{"missing token", request(http.MethodGet, "/api/config", ""), http.StatusUnauthorized},
		{"wrong token", func() *http.Request {
			r := request(http.MethodGet, "/api/config", "")
			r.Header.Set("Authorization", "Bearer not-the-token")
			return r
		}(), http.StatusUnauthorized},
		{"web page origin", func() *http.Request {
			r := authorized(http.MethodGet, "/api/config", "")
			r.Header.Set("Origin", "https://hostile.example")
			return r
		}(), http.StatusForbidden},
		{"bad host", func() *http.Request {
			r := authorized(http.MethodGet, "/api/config", "")
			r.Host = "evil.example:8765"
			return r
		}(), http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := serve(server, tc.req).Code; got != tc.want {
				t.Errorf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestHealthAndExtensionPreflightAreAvailableWithoutBearerToken(t *testing.T) {
	server, _ := newTestServer(t, false)
	health := serve(server, request(http.MethodGet, "/healthz", ""))
	if got := health.Code; got != http.StatusOK {
		t.Fatalf("health status = %d, want 200", got)
	}
	if !strings.Contains(health.Body.String(), `"daemon_id":"0123456789abcdef0123456789abcdef"`) {
		t.Errorf("health response does not identify helper: %s", health.Body.String())
	}
	preflight := request(http.MethodOptions, "/api/analyze", "")
	preflight.Header.Set("Origin", "chrome-extension://abcdefghijklmnop")
	got := serve(server, preflight)
	if got.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", got.Code)
	}
	if got.Header().Get("Access-Control-Allow-Origin") != preflight.Header.Get("Origin") {
		t.Errorf("preflight did not preserve extension origin: %q", got.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestAnalyzeValidatesListingAndReturnsAssessment(t *testing.T) {
	server, _ := newTestServer(t, false)
	valid := `{"marketplace":"craigslist","listing_url":"https://sfbay.craigslist.org/apa/1.html","title":"Send gift cards","description":"Pay by gift card"}`
	response := serve(server, authorized(http.MethodPost, "/api/analyze", valid))
	if response.Code != http.StatusOK {
		t.Fatalf("analyze status = %d: %s", response.Code, response.Body.String())
	}
	var assessment struct {
		RiskScore    float64 `json:"risk_score"`
		NotEvaluated []struct {
			Reason string `json:"reason"`
		} `json:"not_evaluated"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &assessment); err != nil {
		t.Fatalf("decode assessment: %v", err)
	}
	if assessment.RiskScore == 0 || len(assessment.NotEvaluated) == 0 {
		t.Errorf("assessment did not evaluate expected signals: %+v", assessment)
	}
	var rawAssessment map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &rawAssessment); err != nil {
		t.Fatalf("decode raw assessment: %v", err)
	}
	for _, field := range []string{"high_risk", "potentially_risky", "positive_signals", "passed_checks", "not_evaluated"} {
		value, ok := rawAssessment[field]
		if !ok || string(value) == "null" {
			t.Errorf("%s = %s, want JSON array", field, value)
			continue
		}
		var group []json.RawMessage
		if err := json.Unmarshal(value, &group); err != nil {
			t.Errorf("%s is not a JSON array: %v", field, err)
		}
	}
	var traced struct {
		Trace []struct {
			Step    string `json:"step"`
			Message string `json:"message"`
		} `json:"trace"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &traced); err != nil {
		t.Fatalf("decode trace: %v", err)
	}
	if len(traced.Trace) == 0 || traced.Trace[0].Step != "daemon" {
		t.Errorf("response trace = %+v, want daemon activity", traced.Trace)
	}

	invalid := serve(server, authorized(http.MethodPost, "/api/analyze", `{"title":"missing required fields"}`))
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid listing status = %d, want 422", invalid.Code)
	}
}

func TestAnalyzeMirrorsSafeTraceWhenVerbose(t *testing.T) {
	server, _ := newTestServer(t, false)
	var output bytes.Buffer
	server.verbose = true
	server.verboseWriter = &output
	response := serve(server, authorized(http.MethodPost, "/api/analyze", `{"marketplace":"craigslist","listing_url":"https://sfbay.craigslist.org/apa/1.html","title":"Normal listing","description":"No payment request"}`))
	if response.Code != http.StatusOK {
		t.Fatalf("analyze status = %d: %s", response.Code, response.Body.String())
	}
	log := output.String()
	if !strings.Contains(log, "accepted analysis request") || !strings.Contains(log, "completed in") {
		t.Errorf("verbose trace missing expected events: %s", log)
	}
	if strings.Contains(log, testToken) || strings.Contains(log, "Normal listing") {
		t.Errorf("verbose trace exposed a secret or listing text: %s", log)
	}
}

func TestConfigResponseDoesNotExposeBearerOrVisionKey(t *testing.T) {
	server, _ := newTestServer(t, false)
	response := serve(server, authorized(http.MethodGet, "/api/config", ""))
	if response.Code != http.StatusOK {
		t.Fatalf("config status = %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), testToken) || strings.Contains(response.Body.String(), "api_key") {
		t.Errorf("config response exposed a secret: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"google_vision":false`) {
		t.Errorf("provider state missing from safe config: %s", response.Body.String())
	}
}

func TestPutVisionAPIKeyUpdatesOnlyTheAuthenticatedLocalRuntime(t *testing.T) {
	server, _ := newTestServer(t, false)
	unauthorized := serve(server, request(http.MethodPut, "/api/vision", `{"api_key":"test-key"}`))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated Vision update status = %d, want 401", unauthorized.Code)
	}
	var received string
	server.setVisionAPIKey = func(apiKey string) (*signals.Vision, error) {
		received = apiKey
		return signals.NewVision(nil, signals.NopMeter{}, signals.Options{APIKey: apiKey, MonthlyCap: 10}), nil
	}
	response := serve(server, authorized(http.MethodPut, "/api/vision", `{"api_key":"  test-key  "}`))
	if response.Code != http.StatusOK {
		t.Fatalf("Vision update status = %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "test-key") || !strings.Contains(response.Body.String(), `"google_vision":true`) {
		t.Errorf("Vision update response = %s, want safe enabled acknowledgement", response.Body.String())
	}
	if received != "test-key" {
		t.Errorf("Vision setter received %q, want trimmed key", received)
	}
	if server.vision == nil || !server.vision.Enabled() {
		t.Error("Vision runtime was not enabled")
	}
	if _, unavailable := server.eng.Compiled().Unavailable["reverse_image_real_estate"]; unavailable {
		t.Errorf("image rules remain unavailable after configuring Vision: %+v", server.eng.Compiled().Unavailable)
	}
	configResponse := serve(server, authorized(http.MethodGet, "/api/config", ""))
	if configResponse.Code != http.StatusOK || strings.Contains(configResponse.Body.String(), "test-key") || !strings.Contains(configResponse.Body.String(), `"google_vision":true`) {
		t.Errorf("safe config after Vision update = %d %s", configResponse.Code, configResponse.Body.String())
	}
	empty := serve(server, authorized(http.MethodPut, "/api/vision", `{"api_key":" "}`))
	if empty.Code != http.StatusUnprocessableEntity {
		t.Errorf("empty Vision key status = %d, want 422", empty.Code)
	}
}

func TestPutRulesAndExplicitReloadChangeActiveRuleSet(t *testing.T) {
	server, rulesPath := newTestServer(t, false)
	set := oneMarkerRuleSet("FIRST_MARKER")
	body, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	put := serve(server, authorized(http.MethodPut, "/api/rules", string(body)))
	if put.Code != http.StatusOK {
		t.Fatalf("put rules status = %d: %s", put.Code, put.Body.String())
	}
	if raw, err := os.ReadFile(rulesPath); err != nil || !bytes.Contains(raw, []byte("FIRST_MARKER")) {
		t.Errorf("written rules not found: err=%v raw=%s", err, raw)
	}
	if score := analysisScore(t, server, "FIRST_MARKER"); score != .45 {
		t.Errorf("put rules score = %v, want .45", score)
	}

	set = oneMarkerRuleSet("RELOADED_MARKER")
	replacement, err := yamlRules(set)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WriteFileSecure(rulesPath, replacement); err != nil {
		t.Fatal(err)
	}
	reload := serve(server, authorized(http.MethodPost, "/api/reload-rules", ""))
	if reload.Code != http.StatusOK {
		t.Fatalf("reload status = %d: %s", reload.Code, reload.Body.String())
	}
	if score := analysisScore(t, server, "RELOADED_MARKER"); score != .45 {
		t.Errorf("reloaded score = %v, want .45", score)
	}
}

func TestAutoReloadPicksUpExternalRuleEdits(t *testing.T) {
	server, rulesPath := newTestServer(t, true)
	replacement, err := yamlRules(oneMarkerRuleSet("AUTO_RELOAD_MARKER"))
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WriteFileSecure(rulesPath, replacement); err != nil {
		t.Fatal(err)
	}
	changed := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(rulesPath, changed, changed); err != nil {
		t.Fatal(err)
	}
	if score := analysisScore(t, server, "AUTO_RELOAD_MARKER"); score != .45 {
		t.Errorf("auto-reloaded score = %v, want .45", score)
	}
}

func TestRulesSchemaAndInvalidUpdateAreReported(t *testing.T) {
	server, _ := newTestServer(t, false)
	schema := serve(server, authorized(http.MethodGet, "/api/rules/schema", ""))
	if schema.Code != http.StatusOK || !strings.Contains(schema.Body.String(), "payment_no_recourse") || !strings.Contains(schema.Body.String(), rules.TypeMarketRentCheck) || !strings.Contains(schema.Body.String(), rules.TypeRentPriceMismatch) {
		t.Errorf("schema response = %d %s", schema.Code, schema.Body.String())
	}
	invalid := `{"version":"1.0","severities":{"red":0.4,"risk":0.2,"green":0},"risk_bands":{"caution":0.3,"elevated":0.5,"high":0.7},"rules":{}}`
	response := serve(server, authorized(http.MethodPut, "/api/rules", invalid))
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "at least one rule") {
		t.Errorf("invalid update = %d %s", response.Code, response.Body.String())
	}
	after := serve(server, authorized(http.MethodGet, "/api/rules", ""))
	if after.Code != http.StatusOK || !strings.Contains(after.Body.String(), "payment_no_recourse") {
		t.Errorf("invalid update changed active rules = %d %s", after.Code, after.Body.String())
	}
}

func TestRulesJSONRoundTripsEveryReadOnlyRule(t *testing.T) {
	server, _ := newTestServer(t, false)
	before := serve(server, authorized(http.MethodGet, "/api/rules", ""))
	if before.Code != http.StatusOK {
		t.Fatalf("get rules = %d: %s", before.Code, before.Body.String())
	}
	put := serve(server, authorized(http.MethodPut, "/api/rules", before.Body.String()))
	if put.Code != http.StatusOK {
		t.Fatalf("put round trip = %d: %s", put.Code, put.Body.String())
	}
	after := serve(server, authorized(http.MethodGet, "/api/rules", ""))
	if after.Code != http.StatusOK {
		t.Fatalf("get rules after round trip = %d: %s", after.Code, after.Body.String())
	}
	var original, roundTripped map[string]any
	if err := json.Unmarshal(before.Body.Bytes(), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(after.Body.Bytes(), &roundTripped); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(roundTripped, original) {
		t.Errorf("rule set changed after JSON round trip\noriginal: %#v\nround-tripped: %#v", original, roundTripped)
	}
}

func analysisScore(t *testing.T, server *Server, title string) float64 {
	t.Helper()
	payload := `{"marketplace":"craigslist","listing_url":"https://sfbay.craigslist.org/apa/1.html","title":` + strconvQuote(title) + `}`
	response := serve(server, authorized(http.MethodPost, "/api/analyze", payload))
	if response.Code != http.StatusOK {
		t.Fatalf("analyze = %d: %s", response.Code, response.Body.String())
	}
	var assessment struct {
		RiskScore float64 `json:"risk_score"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &assessment); err != nil {
		t.Fatal(err)
	}
	return assessment.RiskScore
}

func oneMarkerRuleSet(marker string) rules.RuleSet {
	return rules.RuleSet{
		Version:    "1.0",
		Severities: rules.Severities{Red: .45, Risk: .20, Green: 0},
		RiskBands:  rules.RiskBands{Caution: .30, Elevated: .55, High: .75},
		Rules: map[string]rules.Rule{
			"marker": {
				Type: rules.TypePatternMatch, Scope: rules.ScopeTitle, Severity: rules.SeverityRed,
				Match: rules.Match{Custom: []string{marker}},
			},
		},
	}
}

func yamlRules(set rules.RuleSet) ([]byte, error) {
	// A JSON round trip would hide YAML-field regressions. The daemon's on-disk
	// contract is YAML, so tests use the same encoder as its PUT endpoint.
	return yaml.Marshal(set)
}

func strconvQuote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func TestFreshAnalysisReachesProvider(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"responses":[{}]}`))
	}))
	defer provider.Close()
	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rulesPath := filepath.Join(t.TempDir(), "rules.yaml")
	if err := config.WriteFileSecure(rulesPath, rules.DefaultRulesYAML()); err != nil {
		t.Fatal(err)
	}
	vision := signals.NewVision(store, nil, signals.Options{APIKey: "test", Endpoint: provider.URL, MonthlyCap: 100})
	server, err := New(Options{Port: 8765, Token: testToken, RulesPath: rulesPath, Vision: vision})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"marketplace":"craigslist","listing_url":"https://sfbay.craigslist.org/apa/1.html","title":"Studio","images":["https://images.craigslist.org/photo.jpg"]}`
	for index, path := range []string{"/api/analyze", "/api/analyze", "/api/analyze?fresh=true"} {
		response := serve(server, authorized(http.MethodPost, path, body))
		if response.Code != 200 {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		want := 2
		if index == 2 {
			want = 4
		}
		if got := int(calls.Load()); got != want {
			t.Fatalf("request %d calls=%d want=%d", index, got, want)
		}
	}
}
