// Package marketplace validates and normalizes listing payloads received from
// browser adapters. It intentionally does not parse marketplace HTML: DOM
// selectors belong in the extension, while this package protects the daemon's
// stable, marketplace-neutral boundary.
package marketplace

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/aidandevv/craig/internal/domain"
)

const maxTextLength = 100_000

// Normalize trims machine-extracted fields, drops empty repeated values and
// rejects payloads that are not a meaningful marketplace listing. Signals see
// one predictable representation regardless of which adapter supplied it.
func Normalize(input domain.Listing) (domain.Listing, error) {
	if len(input.Images) > 24 || len(input.Captions) > 128 {
		return domain.Listing{}, fmt.Errorf("listing has too many images or captions")
	}
	total := len(input.Title) + len(input.Description) + len(input.URL) + len(input.Contact.Email) + len(input.Contact.Phone)
	for _, values := range [][]string{input.Images, input.Captions} {
		for _, value := range values {
			if len(value) > 4096 {
				return domain.Listing{}, fmt.Errorf("image URL or caption exceeds size limit")
			}
			total += len(value)
		}
	}
	if total > 400_000 || len(input.URL) > 4096 {
		return domain.Listing{}, fmt.Errorf("listing exceeds size limit")
	}
	listing := input
	listing.Marketplace = strings.ToLower(strings.TrimSpace(listing.Marketplace))
	listing.URL = strings.TrimSpace(listing.URL)
	listing.Title = strings.TrimSpace(listing.Title)
	listing.Description = strings.TrimSpace(listing.Description)
	listing.Currency = strings.ToUpper(strings.TrimSpace(listing.Currency))
	listing.RentPeriod = strings.ToLower(strings.TrimSpace(listing.RentPeriod))
	listing.ZIPCode = strings.TrimSpace(listing.ZIPCode)
	listing.Contact.Email = strings.TrimSpace(listing.Contact.Email)
	listing.Contact.Phone = strings.TrimSpace(listing.Contact.Phone)

	if listing.Marketplace == "" {
		return domain.Listing{}, fmt.Errorf("marketplace is required")
	}
	if listing.URL == "" {
		return domain.Listing{}, fmt.Errorf("listing_url is required")
	}
	parsed, err := url.ParseRequestURI(listing.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return domain.Listing{}, fmt.Errorf("listing_url must be an absolute http(s) URL")
	}
	if listing.Title == "" {
		return domain.Listing{}, fmt.Errorf("title is required")
	}
	for name, text := range map[string]string{"title": listing.Title, "description": listing.Description} {
		if utf8.RuneCountInString(text) > maxTextLength {
			return domain.Listing{}, fmt.Errorf("%s exceeds %d characters", name, maxTextLength)
		}
	}
	listing.Images = cleanValues(listing.Images)
	listing.Captions = cleanValues(listing.Captions)
	return listing, nil
}

func cleanValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}
	return cleaned
}
