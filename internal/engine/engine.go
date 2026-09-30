// Package engine is the one analysis pipeline every Craig build shares: the
// CLI, the local daemon, and the browser (WebAssembly) build. Transport and
// storage live with the caller; scoring lives here.
package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/aidandevv/craig-extension/internal/detect"
	"github.com/aidandevv/craig-extension/internal/domain"
	"github.com/aidandevv/craig-extension/internal/marketplace"
	"github.com/aidandevv/craig-extension/internal/risk"
	"github.com/aidandevv/craig-extension/internal/rules"
	"github.com/aidandevv/craig-extension/internal/trace"
)

// ErrInvalidListing wraps validation failures so callers can map them to a
// client error rather than an engine failure.
var ErrInvalidListing = errors.New("invalid listing")

// Options tunes a single analysis.
type Options struct {
	// MaxImages caps how many listing photos reach billable image signals.
	// Zero means every photo.
	MaxImages int
}

// Engine is an immutable compiled rule set, safe for concurrent use.
type Engine struct{ compiled rules.Compiled }

func New(set rules.RuleSet, deps rules.Deps) (*Engine, error) {
	compiled, err := rules.Compile(set, deps)
	if err != nil {
		return nil, err
	}
	return &Engine{compiled: compiled}, nil
}

// Compiled exposes the compiled rules for callers that report on them.
func (e *Engine) Compiled() rules.Compiled { return e.compiled }

func (e *Engine) Analyze(ctx context.Context, listing domain.Listing, opts Options) (risk.Assessment, error) {
	listing, err := marketplace.Normalize(listing)
	if err != nil {
		trace.Log(ctx, "listing", "listing validation failed")
		return risk.Assessment{}, fmt.Errorf("%w: %v", ErrInvalidListing, err)
	}
	if opts.MaxImages > 0 && len(listing.Images) > opts.MaxImages {
		trace.Log(ctx, "listing", "checking the first %d of %d image(s)", opts.MaxImages, len(listing.Images))
		listing.Images = listing.Images[:opts.MaxImages]
	}
	trace.Log(ctx, "listing", "normalized listing with %d image(s)", len(listing.Images))
	trace.Log(ctx, "rules", "running %d detector(s) for %d enabled rule(s)", len(e.compiled.Detectors), len(e.compiled.Rules))
	started := time.Now()
	results := detect.Evaluate(ctx, e.compiled.Detectors, listing)
	return risk.Assess(results, e.compiled, time.Since(started)), nil
}

// UpgradeRuleSet validates before migration so defaults cannot repair an invalid payload.
func UpgradeRuleSet(set rules.RuleSet) (rules.RuleSet, error) {
	if err := rules.Validate(set); err != nil {
		return rules.RuleSet{}, err
	}
	for _, migrate := range []func(*rules.RuleSet) (bool, error){
		rules.MigrateApplicationFeeRule,
		rules.MigrateMarketRentRule,
		rules.MigrateRentPriceMismatchRule,
		rules.MigratePrepaymentBeforeAccessRule,
	} {
		if _, err := migrate(&set); err != nil {
			return rules.RuleSet{}, fmt.Errorf("migrate rules: %w", err)
		}
	}
	return set, rules.Validate(set)
}

// PrepareRuleSet upgrades rules and round-trips them through the strict YAML parser.
func PrepareRuleSet(set rules.RuleSet) (rules.RuleSet, []byte, error) {
	set, err := UpgradeRuleSet(set)
	if err != nil {
		return rules.RuleSet{}, nil, err
	}
	data, err := yaml.Marshal(set)
	if err != nil {
		return rules.RuleSet{}, nil, fmt.Errorf("encode rules: %w", err)
	}
	validated, err := rules.Parse(data)
	if err != nil {
		return rules.RuleSet{}, nil, err
	}
	return validated, data, nil
}
