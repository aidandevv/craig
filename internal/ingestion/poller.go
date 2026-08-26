package ingestion

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aidandevaney/apartment-hunter/internal/database"
	"github.com/aidandevaney/apartment-hunter/internal/domain"
	"github.com/aidandevaney/apartment-hunter/internal/metrics"
)

type Processor func(context.Context, domain.Listing) error
type Poller struct {
	client   *Client
	store    *database.Store
	metrics  *metrics.Metrics
	interval time.Duration
	process  Processor
	logger   *slog.Logger
}

func NewPoller(client *Client, store *database.Store, metric *metrics.Metrics, interval time.Duration, process Processor, logger *slog.Logger) *Poller {
	return &Poller{client: client, store: store, metrics: metric, interval: interval, process: process, logger: logger}
}
func (p *Poller) Run(ctx context.Context) {
	if !p.client.Enabled() {
		p.logger.Warn("Flipify poller disabled: FLIPIFY_API_KEY is not configured")
		return
	}
	p.poll(ctx)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}
func (p *Poller) poll(ctx context.Context) {
	listings, err := p.client.Search(ctx)
	if err != nil {
		p.logger.Error("Flipify search failed", "error", err)
		return
	}
	p.metrics.LastPoll.SetToCurrentTime()
	_ = p.store.SetState(ctx, "last_flipify_poll_success", time.Now().UTC().Format(time.RFC3339))
	p.logger.Info("Flipify search completed", "result_count", len(listings))
	for _, listing := range listings {
		id, fresh, err := p.store.InsertDiscovered(ctx, listing)
		if err != nil {
			p.logger.Error("persist discovered listing", "source_id", listing.SourceID, "error", err)
			continue
		}
		if !fresh {
			continue
		}
		p.metrics.Ingested.Inc()
		p.metrics.QueueDepth.Inc()
		enriched, err := p.client.Enrich(ctx, listing.SourceID)
		if err != nil {
			p.logger.Error("enrich listing", "source_id", listing.SourceID, "error", err)
			p.metrics.QueueDepth.Dec()
			continue
		}
		if enriched.SourceID == "" {
			enriched.SourceID = listing.SourceID
		}
		if err := p.store.ApplyEnrichment(ctx, id, enriched); err != nil {
			p.logger.Error("persist enriched listing", "id", id, "error", err)
			p.metrics.QueueDepth.Dec()
			continue
		}
		enriched.ID = id
		if err := p.process(ctx, enriched); err != nil {
			p.logger.Error("process listing", "id", id, "error", err)
		}
		p.metrics.QueueDepth.Dec()
	}
}
func (p *Poller) Preflight(ctx context.Context) error {
	usage, err := p.client.Preflight(ctx)
	if err != nil {
		return err
	}
	p.logger.Info("Flipify allowance preflight", "account", fmt.Sprintf("%v", usage))
	return nil
}
