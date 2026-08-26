package database

import (
	"context"
	"testing"

	"github.com/aidandevaney/apartment-hunter/internal/domain"
)

func TestListingLifecycleAndBudget(t *testing.T) {
	store, err := Open("sqlite:///:memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	bedrooms := 1.0
	id, fresh, err := store.InsertDiscovered(ctx, domain.Listing{SourceID: "opaque-id", Title: "Test apartment", Price: 2500, Bedrooms: &bedrooms, Location: "Mission"})
	if err != nil || !fresh || id == 0 {
		t.Fatalf("insert: id=%d fresh=%v err=%v", id, fresh, err)
	}
	if _, fresh, err = store.InsertDiscovered(ctx, domain.Listing{SourceID: "opaque-id", Title: "Duplicate"}); err != nil || fresh {
		t.Fatalf("duplicate should be ignored: fresh=%v err=%v", fresh, err)
	}
	if err = store.ApplyEnrichment(ctx, id, domain.Listing{Title: "Test apartment", Price: 2500, Bedrooms: &bedrooms, Location: "Mission", Images: []string{"https://example.test/image.jpg"}}); err != nil {
		t.Fatal(err)
	}
	result := domain.FraudResult{Score: .1, FitScore: .9, Qualified: true, Urgent: true, ElapsedMS: 12, Flags: []string{"clean"}}
	if err = store.SaveEvaluation(ctx, id, result); err != nil {
		t.Fatal(err)
	}
	listings, total, err := store.ListQualified(ctx, .5, 1, 25)
	if err != nil || total != 1 || len(listings) != 1 || listings[0].FraudScore == nil || *listings[0].FraudScore != .1 {
		t.Fatalf("qualified=%#v total=%d err=%v", listings, total, err)
	}
	if err = store.Mark(ctx, id, "contacted", "Sent a question"); err != nil {
		t.Fatal(err)
	}
	listing, err := store.Get(ctx, id)
	if err != nil || listing.Status != "contacted" || listing.UserNotes != "Sent a question" {
		t.Fatalf("listing=%#v err=%v", listing, err)
	}
	if allowed, err := store.ReserveVisionUnit(ctx, "google_vision_web_detection", 1); err != nil || !allowed {
		t.Fatalf("first budget reservation allowed=%v err=%v", allowed, err)
	}
	if allowed, err := store.ReserveVisionUnit(ctx, "google_vision_web_detection", 1); err != nil || allowed {
		t.Fatalf("second budget reservation allowed=%v err=%v", allowed, err)
	}
}
