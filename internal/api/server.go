// Package api exposes the Go-owned application API and keeps all SQLite access
// inside this process. Flask simply forwards browser requests to these routes.
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aidandevaney/apartment-hunter/internal/database"
	"github.com/aidandevaney/apartment-hunter/internal/domain"
)

type Server struct {
	store        *database.Store
	qualifiedMax float64
	logger       *slog.Logger
	onCostUpdate func(context.Context)
}

func New(store *database.Store, qualifiedMax float64, logger *slog.Logger) *Server {
	return &Server{store: store, qualifiedMax: qualifiedMax, logger: logger}
}
func (s *Server) SetCostRefresher(callback func(context.Context)) { s.onCostUpdate = callback }
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/listings", s.list)
	mux.HandleFunc("GET /api/listings/", s.listing)
	mux.HandleFunc("POST /api/listings/", s.mark)
	mux.HandleFunc("GET /api/stats", s.stats)
	mux.HandleFunc("POST /api/model-evaluations", s.modelEvaluation)
	mux.HandleFunc("POST /api/costs", s.cost)
	return s.log(mux)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	write(w, http.StatusOK, map[string]string{"status": "ok"})
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	listings, total, err := s.store.ListQualified(r.Context(), s.qualifiedMax, page, perPage)
	if err != nil {
		internalError(w, err)
		return
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}
	write(w, http.StatusOK, map[string]any{"data": listings, "pagination": map[string]int{"page": page, "per_page": perPage, "total": total}})
}
func (s *Server) listing(w http.ResponseWriter, r *http.Request) {
	id, ok := listingID(r.URL.Path)
	if !ok {
		notFound(w)
		return
	}
	listing, err := s.store.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		notFound(w)
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	write(w, http.StatusOK, listing)
}
func (s *Server) mark(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/mark") {
		notFound(w)
		return
	}
	idPath := strings.TrimSuffix(r.URL.Path, "/mark")
	id, ok := listingID(idPath)
	if !ok {
		notFound(w)
		return
	}
	var input struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&input); err != nil {
		write(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if err := s.store.Mark(r.Context(), id, input.Status, input.Notes); errors.Is(err, sql.ErrNoRows) {
		notFound(w)
		return
	} else if err != nil {
		write(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	listing, err := s.store.Get(r.Context(), id)
	if err != nil {
		internalError(w, err)
		return
	}
	write(w, http.StatusOK, listing)
}
func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.Stats(r.Context(), s.qualifiedMax)
	if err != nil {
		internalError(w, err)
		return
	}
	write(w, http.StatusOK, stats)
}
func (s *Server) modelEvaluation(w http.ResponseWriter, r *http.Request) {
	var e domain.ModelEvaluation
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&e); err != nil {
		write(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if e.EvalDate.IsZero() {
		e.EvalDate = time.Now().UTC()
	}
	if e.ModelVersion == "" {
		e.ModelVersion = "unknown"
	}
	if err := s.store.RecordModelEvaluation(r.Context(), e); err != nil {
		internalError(w, err)
		return
	}
	write(w, http.StatusCreated, e)
}
func (s *Server) cost(w http.ResponseWriter, r *http.Request) {
	var entry domain.CostEntry
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&entry); err != nil {
		write(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if err := s.store.RecordCost(r.Context(), entry.Service, entry.Cost, entry.UnitCount, entry.Notes); err != nil {
		write(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if s.onCostUpdate != nil {
		s.onCostUpdate(r.Context())
	}
	write(w, http.StatusCreated, entry)
}
func (s *Server) log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Debug("api request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
	})
}
func listingID(path string) (int64, bool) {
	part := strings.Trim(strings.TrimPrefix(path, "/api/listings"), "/")
	if part == "" || strings.Contains(part, "/") {
		return 0, false
	}
	id, err := strconv.ParseInt(part, 10, 64)
	return id, err == nil && id > 0
}
func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func notFound(w http.ResponseWriter) {
	write(w, http.StatusNotFound, map[string]string{"error": "listing not found"})
}
func internalError(w http.ResponseWriter, err error) {
	write(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
