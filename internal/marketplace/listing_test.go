package marketplace

import (
	"strings"
	"testing"

	"github.com/aidandevv/craig-extension/internal/domain"
)

func TestNormalizeTrimsAndDeduplicatesAdapterOutput(t *testing.T) {
	bedrooms := 2
	got, err := Normalize(domain.Listing{
		Marketplace: " Craigslist ", URL: " https://sfbay.craigslist.org/apa/1.html ",
		Title: "  A home  ", Description: "  Bright and quiet  ", Currency: " usd ", RentPeriod: " Monthly ", Bedrooms: &bedrooms, ZIPCode: " 94103 ",
		Images:   []string{" https://images.example/a.jpg ", "https://images.example/a.jpg", ""},
		Captions: []string{" kitchen ", "kitchen", ""},
		Contact:  domain.Contact{Phone: " 510-555-1234 "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Marketplace != "craigslist" || got.Title != "A home" || got.Currency != "USD" || got.RentPeriod != "monthly" || got.ZIPCode != "94103" || got.Bedrooms == nil || *got.Bedrooms != 2 {
		t.Errorf("normalization = %+v", got)
	}
	if len(got.Images) != 1 || got.Images[0] != "https://images.example/a.jpg" || len(got.Captions) != 1 {
		t.Errorf("repeated values = images:%v captions:%v", got.Images, got.Captions)
	}
}

func TestNormalizeRejectsIncompleteOrUnsafePayloads(t *testing.T) {
	valid := domain.Listing{Marketplace: "craigslist", URL: "https://example.test/1", Title: "Home"}
	for name, mutate := range map[string]func(*domain.Listing){
		"marketplace": func(l *domain.Listing) { l.Marketplace = "" },
		"url":         func(l *domain.Listing) { l.URL = "relative/listing" },
		"title":       func(l *domain.Listing) { l.Title = "" },
		"oversized": func(l *domain.Listing) {
			l.Description = strings.Repeat("x", maxTextLength+1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			listing := valid
			mutate(&listing)
			if _, err := Normalize(listing); err == nil {
				t.Fatal("expected validation failure")
			}
		})
	}
}

func TestNormalizeRejectsOversizedCaptions(t *testing.T) {
	_, err := Normalize(domain.Listing{Marketplace: "craigslist", URL: "https://sfbay.craigslist.org/apa/1.html", Title: "Studio", Captions: []string{strings.Repeat("a", 4097)}})
	if err == nil {
		t.Fatal("oversized caption accepted")
	}
}
