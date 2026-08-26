package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aidandevaney/apartment-hunter/internal/config"
	"github.com/aidandevaney/apartment-hunter/internal/domain"
)

type Client struct {
	baseURL, apiKey, query string
	latitude, longitude    float64
	httpClient             *http.Client
}

func NewClient(c config.Config) *Client {
	return &Client{baseURL: c.FlipifyBaseURL, apiKey: c.FlipifyAPIKey, query: c.FlipifyQuery, latitude: c.FlipifyLatitude, longitude: c.FlipifyLongitude, httpClient: &http.Client{Timeout: c.HTTPTimeout}}
}
func (c *Client) Enabled() bool { return c.apiKey != "" }

// Preflight returns the account's live allowance and usage without guessing
// from plan names. Callers record it in logs before reducing poll frequency.
func (c *Client) Preflight(ctx context.Context) (map[string]any, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("Flipify API key is not configured")
	}
	result := map[string]any{}
	for _, path := range []string{"/account/credits", "/account/usage"} {
		var value any
		if err := c.getJSON(ctx, path, nil, &value); err != nil {
			return nil, err
		}
		result[strings.TrimPrefix(path, "/")] = value
	}
	return result, nil
}

func (c *Client) Search(ctx context.Context) ([]domain.Listing, error) {
	if !c.Enabled() {
		return nil, nil
	}
	page := 1
	all := []domain.Listing{}
	for page <= 20 {
		values := url.Values{"market": []string{"Craigslist"}, "query": []string{c.query}, "latitude": []string{strconv.FormatFloat(c.latitude, 'f', -1, 64)}, "longitude": []string{strconv.FormatFloat(c.longitude, 'f', -1, 64)}, "page": []string{strconv.Itoa(page)}}
		var payload json.RawMessage
		if err := c.getJSON(ctx, "/listings/search", values, &payload); err != nil {
			return nil, err
		}
		listings, next, err := parseSearch(payload)
		if err != nil {
			return nil, err
		}
		all = append(all, listings...)
		if !next {
			break
		}
		page++
	}
	return all, nil
}

func (c *Client) Enrich(ctx context.Context, sourceID string) (domain.Listing, error) {
	if sourceID == "" {
		return domain.Listing{}, fmt.Errorf("source ID is required")
	}
	payload := map[string]any{"fields": []string{"description", "images", "location", "attributes", "contact"}}
	raw, err := json.Marshal(payload)
	if err != nil {
		return domain.Listing{}, err
	}
	var response json.RawMessage
	if err = c.doJSON(ctx, http.MethodPost, "/listings/"+url.PathEscape(sourceID)+"/enrich", nil, bytes.NewReader(raw), &response); err != nil {
		return domain.Listing{}, err
	}
	return parseEnrichment(response)
}

// Snapshot obtains Flipify's no-refresh basic listing view. It is useful for
// debugging and reconciliation; normal intake enriches only fresh IDs.
func (c *Client) Snapshot(ctx context.Context, sourceID string) (domain.Listing, error) {
	if sourceID == "" {
		return domain.Listing{}, fmt.Errorf("source ID is required")
	}
	var response json.RawMessage
	if err := c.getJSON(ctx, "/listings/"+url.PathEscape(sourceID), nil, &response); err != nil {
		return domain.Listing{}, err
	}
	return parseEnrichment(response)
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	return c.doJSON(ctx, http.MethodGet, path, query, nil, out)
}
func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body io.Reader, out any) error {
	endpoint, err := url.Parse(c.baseURL + path)
	if err != nil {
		return err
	}
	endpoint.RawQuery = query.Encode()
	for attempt := 0; attempt < 3; attempt++ {
		requestBody := body
		if body != nil {
			if seeker, ok := body.(io.Seeker); ok {
				_, _ = seeker.Seek(0, io.SeekStart)
			}
		}
		req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), requestBody)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("X-API-Key", c.apiKey)
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			if err := waitRetry(ctx, attempt, ""); err != nil {
				return err
			}
			continue
		}
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out == nil {
				return nil
			}
			if err := json.Unmarshal(data, out); err != nil {
				return fmt.Errorf("decode %s: %w", path, err)
			}
			return nil
		}
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			if err := waitRetry(ctx, attempt, resp.Header.Get("Retry-After")); err != nil {
				return err
			}
			continue
		}
		return fmt.Errorf("Flipify %s returned HTTP %d: %s", path, resp.StatusCode, truncate(string(data), 300))
	}
	return fmt.Errorf("Flipify %s failed after retries", path)
}
func waitRetry(ctx context.Context, attempt int, retryAfter string) error {
	d := time.Duration(1<<attempt) * 250 * time.Millisecond
	if seconds, e := strconv.Atoi(retryAfter); e == nil && seconds > 0 {
		d = time.Duration(seconds) * time.Second
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func parseSearch(payload json.RawMessage) ([]domain.Listing, bool, error) {
	var envelope struct {
		Listings   []rawListing `json:"listings"`
		Data       []rawListing `json:"data"`
		Pagination struct {
			NextPage any  `json:"next_page"`
			HasNext  bool `json:"has_next"`
		} `json:"pagination"`
		NextPage any `json:"next_page"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, false, err
	}
	raws := envelope.Listings
	if len(raws) == 0 {
		raws = envelope.Data
	}
	if len(raws) == 0 {
		var direct []rawListing
		if err := json.Unmarshal(payload, &direct); err == nil {
			raws = direct
		} else if string(payload) != "[]" && string(payload) != "{}" {
			return nil, false, fmt.Errorf("Flipify search response has no listings array")
		}
	}
	result := make([]domain.Listing, 0, len(raws))
	for _, raw := range raws {
		l := normalize(raw)
		if l.SourceID != "" {
			result = append(result, l)
		}
	}
	return result, envelope.Pagination.HasNext || envelope.Pagination.NextPage != nil || envelope.NextPage != nil, nil
}
func parseEnrichment(payload json.RawMessage) (domain.Listing, error) {
	var wrapped struct {
		Listing rawListing `json:"listing"`
		Data    rawListing `json:"data"`
	}
	if err := json.Unmarshal(payload, &wrapped); err != nil {
		return domain.Listing{}, err
	}
	raw := wrapped.Listing
	if raw.ID == "" && raw.ListingID == "" {
		raw = wrapped.Data
	}
	if raw.ID == "" && raw.ListingID == "" {
		if err := json.Unmarshal(payload, &raw); err != nil {
			return domain.Listing{}, err
		}
	}
	return normalize(raw), nil
}
func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "…"
}
