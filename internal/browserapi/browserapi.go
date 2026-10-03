// Package browserapi is the JSON contract between the browser extension and
// the WebAssembly engine. It is plain Go so it can be tested natively; the
// syscall/js wiring lives in cmd/craig-wasm.
package browserapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aidandevv/craig-extension/internal/domain"
	"github.com/aidandevv/craig-extension/internal/engine"
	"github.com/aidandevv/craig-extension/internal/risk"
	"github.com/aidandevv/craig-extension/internal/rules"
	"github.com/aidandevv/craig-extension/internal/signals"
	"github.com/aidandevv/craig-extension/internal/trace"
)

// EvidenceTTL matches the daemon's SQLite cache so both builds re-query
// Vision on the same schedule.
const EvidenceTTL = 24 * time.Hour

const MaxRequestBytes = 1 << 20

type AnalyzeRequest struct {
	Listing   domain.Listing `json:"listing"`
	Rules     *rules.RuleSet `json:"rules,omitempty"`
	Fresh     bool           `json:"fresh,omitempty"`
	CacheOnly bool           `json:"cache_only,omitempty"`
	Vision    VisionSettings `json:"vision"`
}

type VisionSettings struct {
	APIKey     string `json:"api_key,omitempty"`
	MonthlyCap int    `json:"monthly_cap"`
	MaxImages  int    `json:"max_images"`
}

type analyzeResponse struct {
	risk.Assessment
	Trace []trace.Event `json:"trace"`
}

// Host carries what the browser supplies: persistent storage and, in tests
// only, a fake Vision endpoint.
type Host struct {
	Store          signals.EvidenceStore
	VisionEndpoint string
}

func (h Host) Analyze(ctx context.Context, raw []byte) ([]byte, error) {
	var req AnalyzeRequest
	if err := decodeStrict(raw, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if req.Vision.MaxImages < 1 || req.Vision.MaxImages > 24 || req.Vision.MonthlyCap < 1 || req.Vision.MonthlyCap > 1_000_000 {
		return nil, errors.New("invalid Vision limits")
	}
	set, err := h.ruleSet(req.Rules)
	if err != nil {
		return nil, err
	}
	deps := rules.Deps{}
	if req.Vision.APIKey != "" && h.Store != nil {
		deps.Vision = signals.NewVision(h.Store, signals.NopMeter{}, signals.Options{
			Endpoint:   h.VisionEndpoint,
			APIKey:     req.Vision.APIKey,
			MonthlyCap: req.Vision.MonthlyCap,
			Timeout:    10 * time.Second,
		})
	}
	eng, err := engine.New(set, deps)
	if err != nil {
		return nil, fmt.Errorf("compile rules: %w", err)
	}
	recorder := trace.NewRecorder(func(trace.Event) {})
	ctx = trace.WithSink(ctx, recorder)
	if req.CacheOnly {
		ctx = signals.WithCachedImages(ctx)
	}
	if req.Fresh {
		ctx = signals.WithFreshImages(ctx)
	}
	assessment, err := eng.Analyze(ctx, req.Listing, engine.Options{MaxImages: req.Vision.MaxImages})
	if err != nil {
		return nil, err
	}
	return json.Marshal(analyzeResponse{Assessment: assessment, Trace: recorder.Events()})
}

func (h Host) ruleSet(stored *rules.RuleSet) (rules.RuleSet, error) {
	if stored == nil {
		return rules.DefaultRuleSet()
	}
	return engine.UpgradeRuleSet(*stored)
}

func PrepareRules(raw []byte) ([]byte, error) {
	var candidate rules.RuleSet
	if err := decodeStrict(raw, &candidate); err != nil {
		return nil, fmt.Errorf("invalid rules JSON: %w", err)
	}
	set, _, err := engine.PrepareRuleSet(candidate)
	if err != nil {
		return nil, err
	}
	if _, err := engine.New(set, rules.Deps{}); err != nil {
		return nil, err
	}
	return json.Marshal(set)
}

func DefaultRules() ([]byte, error) {
	set, err := rules.DefaultRuleSet()
	if err != nil {
		return nil, err
	}
	return json.Marshal(set)
}

func RuleSchema() ([]byte, error) { return json.Marshal(rules.Schema()) }

type storedEvidence struct {
	Response  string    `json:"response"`
	CheckedAt time.Time `json:"checked_at"`
}

func EncodeEvidence(data map[string]any) (string, error) {
	raw, err := json.Marshal(data)
	return string(raw), err
}

// DecodeEvidence treats anything malformed or expired as a cache miss, so a
// corrupted storage entry costs one Vision call rather than an analysis.
func DecodeEvidence(raw string, now time.Time) (map[string]any, time.Time, bool) {
	var stored storedEvidence
	if json.Unmarshal([]byte(raw), &stored) != nil || stored.CheckedAt.IsZero() || now.Sub(stored.CheckedAt) > EvidenceTTL {
		return nil, time.Time{}, false
	}
	var data map[string]any
	if json.Unmarshal([]byte(stored.Response), &data) != nil {
		return nil, time.Time{}, false
	}
	return data, stored.CheckedAt, true
}

func decodeStrict(raw []byte, out any) error {
	if len(raw) > MaxRequestBytes {
		return errors.New("request exceeds size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing data after JSON value")
	}
	return nil
}
