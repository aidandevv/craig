// Package daemon exposes the local analysis engine to the browser extension.
// Its request guard is intentionally stricter than a typical localhost server:
// every site a user visits can attempt to call localhost, so localhost alone is
// not an authorization boundary.
package daemon

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/aidandevv/craig/internal/config"
	"github.com/aidandevv/craig/internal/domain"
	"github.com/aidandevv/craig/internal/engine"
	"github.com/aidandevv/craig/internal/risk"
	"github.com/aidandevv/craig/internal/rules"
	"github.com/aidandevv/craig/internal/signals"
	"github.com/aidandevv/craig/internal/trace"
)

const maxRequestBody = 1 << 20 // Listing payloads should be small, never image bytes.

const maxVisionKeyRequestBody = 4 << 10

// Options contains the runtime collaborators that must be created outside the
// HTTP layer. In particular, the cache and Vision client have one shared
// lifetime across all requests.
type Options struct {
	Port            int
	Token           string
	DaemonID        string
	RulesPath       string
	AutoReload      bool
	CacheLocation   string
	Vision          *signals.Vision
	SetVisionAPIKey func(string) (*signals.Vision, error)
	Verbose         bool
	VerboseWriter   io.Writer
}

// Server is an http.Handler with a swappable compiled ruleset. A failed reload
// never replaces a last-known-good set.
type Server struct {
	port            int
	token           string
	daemonID        string
	rulesPath       string
	autoReload      bool
	cacheLocation   string
	deps            rules.Deps
	vision          *signals.Vision
	setVisionAPIKey func(string) (*signals.Vision, error)
	verbose         bool
	verboseWriter   io.Writer

	mu             sync.RWMutex
	visionUpdateMu sync.Mutex
	set            rules.RuleSet
	eng            *engine.Engine
	rulesModTime   time.Time
	rulesSize      int64
}

// New loads and compiles the first ruleset. A daemon that cannot validate its
// rules does not start: otherwise a user could assume stale or partial rules
// were protecting them.
func New(opts Options) (*Server, error) {
	if opts.Port < 1 || opts.Port > 65535 {
		return nil, fmt.Errorf("daemon port must be in [1,65535]")
	}
	if len(opts.Token) == 0 {
		return nil, errors.New("daemon bearer token is required")
	}
	if strings.TrimSpace(opts.RulesPath) == "" {
		return nil, errors.New("rules path is required")
	}
	set, err := loadRulesWithMigrations(opts.RulesPath)
	if err != nil {
		return nil, err
	}
	eng, err := engine.New(set, rules.Deps{Vision: opts.Vision})
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(opts.RulesPath)
	if err != nil {
		return nil, fmt.Errorf("stat rules file: %w", err)
	}
	return &Server{
		port: opts.Port, token: opts.Token, daemonID: opts.DaemonID, rulesPath: opts.RulesPath,
		autoReload: opts.AutoReload, cacheLocation: opts.CacheLocation,
		deps: rules.Deps{Vision: opts.Vision}, vision: opts.Vision,
		setVisionAPIKey: opts.SetVisionAPIKey,
		verbose:         opts.Verbose, verboseWriter: opts.VerboseWriter, set: set, eng: eng,
		rulesModTime: info.ModTime(), rulesSize: info.Size(),
	}, nil
}

// ServeHTTP implements the complete daemon API. Host and Origin checks run
// before endpoint routing, including healthz, to avoid making this process a
// useful DNS-rebinding target.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.allowedHost(r.Host) {
		writeError(w, http.StatusForbidden, "host is not allowed")
		return
	}
	if !allowedOrigin(r.Header.Get("Origin")) {
		writeError(w, http.StatusForbidden, "origin is not allowed")
		return
	}
	if r.Method == http.MethodOptions && isAPIPath(r.URL.Path) {
		s.writePreflight(w, r)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}

	switch r.URL.Path {
	case "/healthz":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "daemon_id": s.daemonID})
	case "/api/analyze":
		if !s.requireAuth(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		s.analyze(w, r)
	case "/api/config":
		if !s.requireAuth(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.getConfig(w)
	case "/api/vision":
		if !s.requireAuth(w, r) {
			return
		}
		if r.Method != http.MethodPut {
			methodNotAllowed(w, http.MethodPut)
			return
		}
		s.putVisionAPIKey(w, r)
	case "/api/rules":
		if !s.requireAuth(w, r) {
			return
		}
		s.rulesEndpoint(w, r)
	case "/api/rules/schema":
		if !s.requireAuth(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.getSchema(w)
	case "/api/reload-rules":
		if !s.requireAuth(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		if err := s.reload(); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "reload rules: "+err.Error())
			return
		}
		s.mu.RLock()
		version := s.set.Version
		s.mu.RUnlock()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": version})
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func isAPIPath(path string) bool {
	return path == "/api/analyze" || path == "/api/config" || path == "/api/vision" || path == "/api/rules" ||
		path == "/api/rules/schema" || path == "/api/reload-rules"
}

func (s *Server) allowedHost(host string) bool {
	name, port, err := net.SplitHostPort(host)
	if err != nil || port != fmt.Sprint(s.port) {
		return false
	}
	return name == "127.0.0.1" || strings.EqualFold(name, "localhost")
}

func allowedOrigin(raw string) bool {
	if raw == "" {
		return true // CLI callers do not send Origin.
	}
	origin, err := url.Parse(raw)
	return err == nil && origin.Scheme == "chrome-extension" && origin.Host != ""
}

func (s *Server) writePreflight(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	value := r.Header.Get("Authorization")
	provided, ok := strings.CutPrefix(value, "Bearer ")
	if !ok || provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) != 1 {
		w.Header().Set("WWW-Authenticate", `Bearer realm="craig-extension"`)
		writeError(w, http.StatusUnauthorized, "valid bearer token required")
		return false
	}
	return true
}

func (s *Server) analyze(w http.ResponseWriter, r *http.Request) {
	recorder := trace.NewRecorder(s.writeTraceEvent)
	ctx := trace.WithSink(r.Context(), recorder)
	if r.URL.Query().Get("fresh") == "true" {
		ctx = signals.WithFreshImages(ctx)
	}
	trace.Log(ctx, "daemon", "accepted analysis request")
	if err := s.maybeReload(); err != nil {
		trace.Log(ctx, "rules", "reload failed; last known-good rules remain active")
		writeAnalysisError(w, http.StatusServiceUnavailable, "rules changed but could not be reloaded: "+err.Error(), recorder.Events())
		return
	}
	var listing domain.Listing
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&listing); err != nil {
		trace.Log(ctx, "listing", "request JSON could not be decoded")
		writeAnalysisError(w, http.StatusBadRequest, "invalid listing JSON: "+err.Error(), recorder.Events())
		return
	}
	if err := requireEOF(decoder); err != nil {
		trace.Log(ctx, "listing", "request JSON had trailing data")
		writeAnalysisError(w, http.StatusBadRequest, "invalid listing JSON: "+err.Error(), recorder.Events())
		return
	}
	s.mu.RLock()
	eng := s.eng
	s.mu.RUnlock()
	assessment, err := eng.Analyze(ctx, listing, engine.Options{})
	if err != nil {
		if errors.Is(err, engine.ErrInvalidListing) {
			// eng.Analyze already logged the "listing validation failed" trace
			// event, and err already carries the "invalid listing: " prefix.
			writeAnalysisError(w, http.StatusUnprocessableEntity, err.Error(), recorder.Events())
			return
		}
		writeAnalysisError(w, http.StatusUnprocessableEntity, "invalid listing: "+err.Error(), recorder.Events())
		return
	}
	trace.Log(ctx, "assessment", "completed in %dms; %d of %d checks ran", assessment.AnalysisTimeMS, assessment.Coverage.Ran, assessment.Coverage.Enabled)
	writeJSON(w, http.StatusOK, struct {
		risk.Assessment
		Trace []trace.Event `json:"trace"`
	}{Assessment: assessment, Trace: recorder.Events()})
}

func (s *Server) writeTraceEvent(event trace.Event) {
	if !s.verbose || s.verboseWriter == nil {
		return
	}
	_, _ = fmt.Fprintf(s.verboseWriter, "[%s] %-10s %s\n", event.Timestamp.Format("15:04:05.000"), event.Step, event.Message)
}

func (s *Server) getConfig(w http.ResponseWriter) {
	s.mu.RLock()
	version := s.set.Version
	enabled := len(s.eng.Compiled().Rules)
	vision := s.vision
	s.mu.RUnlock()
	response := struct {
		DaemonID     string `json:"daemon_id"`
		RulesVersion string `json:"rules_version"`
		Providers    struct {
			GoogleVision bool `json:"google_vision"`
		} `json:"providers"`
		CacheLocation    string `json:"cache_location"`
		EnabledRuleCount int    `json:"enabled_rule_count"`
	}{
		DaemonID: s.daemonID, RulesVersion: version, CacheLocation: s.cacheLocation, EnabledRuleCount: enabled,
	}
	response.Providers.GoogleVision = vision != nil && vision.Enabled()
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) putVisionAPIKey(w http.ResponseWriter, r *http.Request) {
	if s.setVisionAPIKey == nil {
		writeError(w, http.StatusNotImplemented, "this daemon cannot update Vision settings")
		return
	}
	var request struct {
		APIKey string `json:"api_key"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxVisionKeyRequestBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid Vision settings JSON: "+err.Error())
		return
	}
	if err := requireEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "invalid Vision settings JSON: "+err.Error())
		return
	}
	if strings.TrimSpace(request.APIKey) == "" {
		writeError(w, http.StatusUnprocessableEntity, "Google Vision API key is required")
		return
	}

	// Serialize config writes and the accompanying in-memory swap. This avoids
	// a later request persisting one key while an earlier request wins in memory.
	s.visionUpdateMu.Lock()
	defer s.visionUpdateMu.Unlock()
	vision, err := s.setVisionAPIKey(strings.TrimSpace(request.APIKey))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "save Vision settings: "+err.Error())
		return
	}
	s.mu.Lock()
	deps := rules.Deps{Vision: vision}
	eng, err := engine.New(s.set, deps)
	if err == nil {
		s.deps, s.vision, s.eng = deps, vision, eng
	}
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "activate Vision settings: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "google_vision": vision != nil && vision.Enabled()})
}

func (s *Server) rulesEndpoint(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		set := s.set
		s.mu.RUnlock()
		writeJSON(w, http.StatusOK, set)
	case http.MethodPut:
		s.putRules(w, r)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPut)
	}
}

func (s *Server) putRules(w http.ResponseWriter, r *http.Request) {
	var candidate rules.RuleSet
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&candidate); err != nil {
		writeError(w, http.StatusBadRequest, "invalid rules JSON: "+err.Error())
		return
	}
	if err := requireEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "invalid rules JSON: "+err.Error())
		return
	}
	// Reject an invalid user payload before adding upgrade defaults, and apply
	// the same migrations and strict round-trip the CLI and browser build use.
	// Otherwise an empty rule set could become valid merely because a newer
	// binary has more migrations than the client knew about.
	validated, data, err := engine.PrepareRuleSet(candidate)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	// Keep the compiled rules paired with the exact Vision dependency that was
	// active when this on-disk update was accepted. A concurrent key update must
	// not leave an enabled provider paired with a no-key compiled snapshot.
	s.mu.Lock()
	defer s.mu.Unlock()
	eng, err := engine.New(validated, s.deps)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := config.WriteFileSecure(s.rulesPath, data); err != nil {
		writeError(w, http.StatusInternalServerError, "write rules: "+err.Error())
		return
	}
	info, err := os.Stat(s.rulesPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "stat written rules: "+err.Error())
		return
	}
	s.set, s.eng, s.rulesModTime, s.rulesSize = validated, eng, info.ModTime(), info.Size()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": validated.Version})
}

func (s *Server) getSchema(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, rules.Schema())
}

func (s *Server) maybeReload() error {
	if !s.autoReload {
		return nil
	}
	info, err := os.Stat(s.rulesPath)
	if err != nil {
		return err
	}
	s.mu.RLock()
	changed := !info.ModTime().Equal(s.rulesModTime) || info.Size() != s.rulesSize
	s.mu.RUnlock()
	if !changed {
		return nil
	}
	return s.reload()
}

func (s *Server) reload() error {
	set, err := loadRulesWithMigrations(s.rulesPath)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	eng, err := engine.New(set, s.deps)
	if err != nil {
		return err
	}
	info, err := os.Stat(s.rulesPath)
	if err != nil {
		return err
	}
	s.set, s.eng, s.rulesModTime, s.rulesSize = set, eng, info.ModTime(), info.Size()
	return nil
}

func loadRulesWithMigrations(path string) (rules.RuleSet, error) {
	set, err := rules.Load(path)
	if err != nil {
		return rules.RuleSet{}, err
	}
	feeChanged, err := rules.MigrateApplicationFeeRule(&set)
	if err != nil {
		return rules.RuleSet{}, fmt.Errorf("migrate rules: %w", err)
	}
	marketRentChanged, err := rules.MigrateMarketRentRule(&set)
	if err != nil {
		return rules.RuleSet{}, fmt.Errorf("migrate rules: %w", err)
	}
	priceMismatchChanged, err := rules.MigrateRentPriceMismatchRule(&set)
	if err != nil {
		return rules.RuleSet{}, fmt.Errorf("migrate rules: %w", err)
	}
	prepaymentChanged, err := rules.MigratePrepaymentBeforeAccessRule(&set)
	if err != nil {
		return rules.RuleSet{}, fmt.Errorf("migrate rules: %w", err)
	}
	if !feeChanged && !marketRentChanged && !priceMismatchChanged && !prepaymentChanged {
		return set, nil
	}
	data, err := yaml.Marshal(set)
	if err != nil {
		return rules.RuleSet{}, fmt.Errorf("encode migrated rules: %w", err)
	}
	if err := config.WriteFileSecure(path, data); err != nil {
		return rules.RuleSet{}, fmt.Errorf("write migrated rules: %w", err)
	}
	return set, nil
}

func requireEOF(decoder *json.Decoder) error {
	var value any
	err := decoder.Decode(&value)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("request must contain one JSON value")
	}
	return err
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeAnalysisError(w http.ResponseWriter, status int, message string, events []trace.Event) {
	writeJSON(w, status, struct {
		Error string        `json:"error"`
		Trace []trace.Event `json:"trace"`
	}{Error: message, Trace: events})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
