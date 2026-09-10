package cache

import (
	"context"
	"testing"
	"time"

	"github.com/aidandevv/craig-extension/internal/domain"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestGetMissReturnsNotFound(t *testing.T) {
	store := newStore(t)
	_, found, err := store.Get(context.Background(), "abc", "reverse_image")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if found {
		t.Error("empty cache reported a hit")
	}
}

func TestPutThenGetRoundTrips(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	want := domain.SignalResult{
		Name:    "reverse_image",
		Risk:    0.6,
		Hard:    true,
		Flags:   []string{"reverse_image_real_estate"},
		Details: []string{"matched zillow.com"},
	}
	if err := store.Put(ctx, "abc", "reverse_image", want); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, found, err := store.Get(ctx, "abc", "reverse_image")
	if err != nil || !found {
		t.Fatalf("get: found=%v err=%v", found, err)
	}
	if got.Risk != want.Risk || got.Hard != want.Hard || len(got.Flags) != 1 || got.Flags[0] != want.Flags[0] {
		t.Errorf("round trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestPutDoesNotCacheSkippedResults(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	skipped := domain.SignalResult{Name: "reverse_image", Skipped: domain.SkipBudgetExhausted}
	if err := store.Put(ctx, "abc", "reverse_image", skipped); err != nil {
		t.Fatalf("put: %v", err)
	}
	if _, found, _ := store.Get(ctx, "abc", "reverse_image"); found {
		t.Error("a skipped signal was cached; it must be retried instead")
	}
}

func TestReserveVisionUnitEnforcesCap(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		allowed, err := store.ReserveVisionUnit(ctx, "web_detection", 3)
		if err != nil {
			t.Fatalf("reserve %d: %v", i, err)
		}
		if !allowed {
			t.Fatalf("reservation %d denied below the cap", i)
		}
	}
	allowed, err := store.ReserveVisionUnit(ctx, "web_detection", 3)
	if err != nil {
		t.Fatalf("reserve past cap: %v", err)
	}
	if allowed {
		t.Error("reservation allowed past the monthly cap")
	}
}

func TestReserveVisionUnitBudgetsFeaturesIndependently(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	if _, err := store.ReserveVisionUnit(ctx, "web_detection", 1); err != nil {
		t.Fatalf("reserve web: %v", err)
	}
	allowed, err := store.ReserveVisionUnit(ctx, "text_detection", 1)
	if err != nil {
		t.Fatalf("reserve ocr: %v", err)
	}
	if !allowed {
		t.Error("exhausting one feature's budget wrongly blocked another")
	}
}

func TestUnitsUsedCountsThisMonth(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if _, err := store.ReserveVisionUnit(ctx, "web_detection", 10); err != nil {
			t.Fatalf("reserve: %v", err)
		}
	}
	used, err := store.UnitsUsed(ctx, "web_detection")
	if err != nil {
		t.Fatalf("units used: %v", err)
	}
	if used != 2 {
		t.Errorf("got %d units used, want 2", used)
	}
}

func TestPruneRemovesStaleEntries(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	if err := store.Put(ctx, "abc", "reverse_image", domain.SignalResult{Name: "reverse_image"}); err != nil {
		t.Fatalf("put: %v", err)
	}
	// Nothing is old enough yet.
	if removed, err := store.Prune(ctx, time.Hour); err != nil || removed != 0 {
		t.Fatalf("premature prune: removed=%d err=%v", removed, err)
	}
	// A negative age puts the cutoff in the future, so everything is stale.
	removed, err := store.Prune(ctx, -time.Hour)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 1 {
		t.Errorf("got %d rows pruned, want 1", removed)
	}
	if _, found, _ := store.Get(ctx, "abc", "reverse_image"); found {
		t.Error("pruned entry still present")
	}
}

func TestOpenRejectsEmptyPath(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Error("expected an error for an empty cache path")
	}
}
