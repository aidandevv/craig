package notifications

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aidandevaney/apartment-hunter/internal/config"
	"github.com/aidandevaney/apartment-hunter/internal/database"
	"github.com/aidandevaney/apartment-hunter/internal/domain"
	"github.com/aidandevaney/apartment-hunter/internal/metrics"
)

func TestDigestContainsOnlySafeReviewFieldsAndIsDeduplicated(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Idempotency-Key") == "" {
			t.Error("missing idempotency key")
		}
		body, _ := io.ReadAll(r.Body)
		text := string(body)
		if strings.Contains(text, "secret@example.test") || strings.Contains(text, "Private listing description") {
			t.Errorf("digest leaked sensitive listing content: %s", text)
		}
		_, _ = io.WriteString(w, `{"id":"resend-message-1"}`)
	}))
	defer server.Close()
	store, err := database.Open("sqlite:///:memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	id, _, err := store.InsertDiscovered(context.Background(), domain.Listing{SourceID: "opaque-1", Title: "Safe title", Price: 2500, Location: "Mission", URL: "https://example.test/listing"})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.ApplyEnrichment(context.Background(), id, domain.Listing{Title: "Safe title", Price: 2500, Location: "Mission", Description: "Private listing description", ContactEmail: "secret@example.test", URL: "https://example.test/listing"}); err != nil {
		t.Fatal(err)
	}
	if err = store.SaveEvaluation(context.Background(), id, domain.FraudResult{Score: .1, FitScore: .9, Qualified: true}); err != nil {
		t.Fatal(err)
	}
	service := New(store, config.Config{ResendAPIKey: "key", ResendAPIURL: server.URL, NotificationEmailFrom: "alerts@example.test", NotificationEmailTo: "owner@example.test", ResendDigestInterval: time.Hour, HTTPTimeout: time.Second, QualifiedMaxFraudScore: .5}, metrics.New(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SendDigest(context.Background())
	service.SendDigest(context.Background())
	if requests != 1 {
		t.Fatalf("requests=%d, want exactly one", requests)
	}
}
