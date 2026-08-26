package ingestion

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/aidandevaney/apartment-hunter/internal/domain"
)

type rawListing struct {
	ID          string          `json:"id"`
	ListingID   string          `json:"listing_id"`
	Title       string          `json:"title"`
	Price       json.RawMessage `json:"price"`
	Bedrooms    json.RawMessage `json:"bedrooms"`
	Bathrooms   json.RawMessage `json:"bathrooms"`
	Location    json.RawMessage `json:"location"`
	Description string          `json:"description"`
	URL         string          `json:"url"`
	ListingURL  string          `json:"listing_url"`
	PostedAt    string          `json:"posted_at"`
	Images      json.RawMessage `json:"images"`
	Attributes  map[string]any  `json:"attributes"`
	Contact     map[string]any  `json:"contact"`
}

func normalize(raw rawListing) domain.Listing {
	location := rawString(raw.Location)
	if location == "" {
		if value, ok := raw.Attributes["location"].(string); ok {
			location = value
		}
	}
	var posted *time.Time
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if value, err := time.Parse(layout, raw.PostedAt); err == nil {
			posted = &value
			break
		}
	}
	id := raw.ID
	if id == "" {
		id = raw.ListingID
	}
	url := raw.URL
	if url == "" {
		url = raw.ListingURL
	}
	l := domain.Listing{SourceID: id, Title: raw.Title, Price: rawInt(raw.Price), Bedrooms: rawFloatPointer(raw.Bedrooms), Bathrooms: rawFloatPointer(raw.Bathrooms), Location: location, Description: raw.Description, URL: url, PostedAt: posted, Images: rawImages(raw.Images)}
	if l.Bedrooms == nil {
		if value, ok := raw.Attributes["bedrooms"]; ok {
			l.Bedrooms = valueFloatPointer(value)
		}
	}
	if l.Bathrooms == nil {
		if value, ok := raw.Attributes["bathrooms"]; ok {
			l.Bathrooms = valueFloatPointer(value)
		}
	}
	if value, ok := raw.Contact["email"].(string); ok {
		l.ContactEmail = value
	}
	if value, ok := raw.Contact["phone"].(string); ok {
		l.ContactPhone = value
	}
	return l
}

func rawString(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	var object map[string]any
	if json.Unmarshal(raw, &object) == nil {
		for _, key := range []string{"name", "display_name", "city", "neighborhood"} {
			if v, ok := object[key].(string); ok {
				return v
			}
		}
	}
	return ""
}
func rawInt(raw json.RawMessage) int {
	if p := rawFloatPointer(raw); p != nil {
		return int(*p)
	}
	return 0
}
func rawFloatPointer(raw json.RawMessage) *float64 {
	if len(raw) == 0 {
		return nil
	}
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return &n
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		s = strings.Trim(strings.ReplaceAll(strings.ReplaceAll(s, "$", ""), ",", ""), " ")
		if n, e := strconv.ParseFloat(s, 64); e == nil {
			return &n
		}
	}
	return nil
}
func valueFloatPointer(value any) *float64 {
	switch v := value.(type) {
	case float64:
		return &v
	case string:
		n, e := strconv.ParseFloat(v, 64)
		if e == nil {
			return &n
		}
	}
	return nil
}
func rawImages(raw json.RawMessage) []string {
	var stringsList []string
	if json.Unmarshal(raw, &stringsList) == nil {
		return stringsList
	}
	var objects []struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(raw, &objects) == nil {
		out := make([]string, 0, len(objects))
		for _, v := range objects {
			if v.URL != "" {
				out = append(out, v.URL)
			}
		}
		return out
	}
	return nil
}
