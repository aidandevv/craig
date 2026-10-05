package engine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"

	"github.com/aidandevv/craig/internal/cache"
	"github.com/aidandevv/craig/internal/domain"
	"github.com/aidandevv/craig/internal/rules"
	"github.com/aidandevv/craig/internal/signals"
)

func scamListing() domain.Listing {
	return domain.Listing{
		Marketplace: "craigslist",
		URL:         "https://sfbay.craigslist.org/apa/1.html",
		Title:       "Beautiful 2BR - MUST GO TODAY",
		Description: "I am out of the country. Send the deposit by western union to hold the unit.",
		Contact:     domain.Contact{RelayOnly: true},
	}
}

func defaultEngine(t *testing.T, deps rules.Deps) *Engine {
	t.Helper()
	set, err := rules.DefaultRuleSet()
	if err != nil {
		t.Fatal(err)
	}
	e, err := New(set, deps)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestAnalyzeScoresScamOfflineAndReportsMissingKey(t *testing.T) {
	assessment, err := defaultEngine(t, rules.Deps{}).Analyze(context.Background(), scamListing(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !assessment.HardFlagged {
		t.Errorf("want hard flag, got %+v", assessment)
	}
	found := false
	for _, row := range assessment.NotEvaluated {
		found = found || row.Reason == domain.SkipNoAPIKey
	}
	if !found {
		t.Errorf("want a %q not-evaluated row, got %+v", domain.SkipNoAPIKey, assessment.NotEvaluated)
	}
}

func TestAnalyzeRejectsInvalidListing(t *testing.T) {
	_, err := defaultEngine(t, rules.Deps{}).Analyze(context.Background(), domain.Listing{}, Options{})
	if !errors.Is(err, ErrInvalidListing) {
		t.Fatalf("err = %v, want ErrInvalidListing", err)
	}
}

func TestAnalyzeMaxImagesLimitsBillablePhotos(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Requests []struct {
				Image struct {
					Source struct {
						ImageURI string `json:"imageUri"`
					} `json:"source"`
				} `json:"image"`
			} `json:"requests"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		for _, req := range body.Requests {
			seen[req.Image.Source.ImageURI] = true
		}
		mu.Unlock()
		_, _ = w.Write([]byte(`{"responses":[{}]}`))
	}))
	t.Cleanup(server.Close)
	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	vision := signals.NewVision(store, signals.NopMeter{}, signals.Options{Endpoint: server.URL, APIKey: "k", MonthlyCap: 100})

	listing := scamListing()
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		listing.Images = append(listing.Images, "https://images.craigslist.org/"+name+".jpg")
	}
	if _, err := defaultEngine(t, rules.Deps{Vision: vision}).Analyze(context.Background(), listing, Options{MaxImages: 2}); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(seen))
	for url := range seen {
		got = append(got, url)
	}
	sort.Strings(got)
	want := []string{"https://images.craigslist.org/a.jpg", "https://images.craigslist.org/b.jpg"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Vision saw %v, want only %v", got, want)
	}
}

func TestPrepareRuleSetRejectsEmptySetBeforeMigrating(t *testing.T) {
	if _, _, err := PrepareRuleSet(rules.RuleSet{}); err == nil {
		t.Fatal("empty rule set must be rejected, not filled in by migrations")
	}
}

func TestPrepareRuleSetAcceptsDefaults(t *testing.T) {
	set, err := rules.DefaultRuleSet()
	if err != nil {
		t.Fatal(err)
	}
	prepared, data, err := PrepareRuleSet(set)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || len(prepared.Rules) != len(set.Rules) {
		t.Errorf("prepared %d rules (%d bytes), want %d", len(prepared.Rules), len(data), len(set.Rules))
	}
}
