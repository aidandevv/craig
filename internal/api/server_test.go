package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aidandevaney/apartment-hunter/internal/database"
	"github.com/aidandevaney/apartment-hunter/internal/domain"
)

func TestListingRoutesRemainWithinGoAPI(t *testing.T) {
	store, err := database.Open("sqlite:///:memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	id, _, err := store.InsertDiscovered(context.Background(), domain.Listing{SourceID: "opaque-1", Title: "One bed", Price: 2500})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SaveEvaluation(context.Background(), id, domain.FraudResult{Score: .1, FitScore: .8, Qualified: true}); err != nil {
		t.Fatal(err)
	}
	handler := New(store, .5, slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))).Handler()
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/listings", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", get.Code, get.Body.String())
	}
	var page struct {
		Data []domain.Listing `json:"data"`
	}
	if err := json.NewDecoder(get.Body).Decode(&page); err != nil || len(page.Data) != 1 {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	mark := httptest.NewRecorder()
	handler.ServeHTTP(mark, httptest.NewRequest(http.MethodPost, "/api/listings/1/mark", bytes.NewBufferString(`{"status":"passed","notes":"not a fit"}`)))
	if mark.Code != http.StatusOK {
		t.Fatalf("POST status=%d body=%s", mark.Code, mark.Body.String())
	}
}
