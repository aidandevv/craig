package ingestion

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aidandevaney/apartment-hunter/internal/config"
)

func TestParseSearchSupportsEnvelope(t *testing.T) {
	items, next, err := parseSearch([]byte(`{"listings":[{"id":"opaque-42","title":"One bedroom","price":"2,500","images":[{"url":"https://example.test/a.jpg"}]}],"pagination":{"has_next":false}}`))
	if err != nil || next || len(items) != 1 || items[0].SourceID != "opaque-42" || items[0].Price != 2500 || len(items[0].Images) != 1 {
		t.Fatalf("items=%#v next=%v err=%v", items, next, err)
	}
}

func TestClientSearchAndEnrichUseFlipifyV1Contract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("missing authentication headers")
		}
		switch r.URL.Path {
		case "/listings/search":
			if r.URL.Query().Get("market") != "Craigslist" || r.URL.Query().Get("latitude") != "37.7" {
				t.Errorf("unexpected search query: %s", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `{"listings":[{"id":"opaque-1","title":"One bedroom","price":2500}]}`)
		case "/listings/opaque-1/enrich":
			if r.Method != http.MethodPost {
				t.Errorf("method=%s", r.Method)
			}
			_, _ = io.WriteString(w, `{"listing":{"id":"opaque-1","title":"One bedroom","price":2500,"images":[{"url":"https://example.test/image.jpg"}],"location":"Mission"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewClient(config.Config{FlipifyBaseURL: server.URL, FlipifyAPIKey: "test-key", FlipifyQuery: "one bedroom", FlipifyLatitude: 37.7, FlipifyLongitude: -122.4, HTTPTimeout: 2 * time.Second})
	listings, err := client.Search(context.Background())
	if err != nil || len(listings) != 1 || listings[0].SourceID != "opaque-1" {
		t.Fatalf("search listings=%#v err=%v", listings, err)
	}
	enriched, err := client.Enrich(context.Background(), listings[0].SourceID)
	if err != nil || len(enriched.Images) != 1 || enriched.Location != "Mission" {
		t.Fatalf("enriched=%#v err=%v", enriched, err)
	}
}
