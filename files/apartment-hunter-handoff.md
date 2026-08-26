# Apartment Hunter: Complete Implementation Handoff

**Project Owner:** Aidan DeVaney  
**Status:** Ready for implementation (MVP phase)  
**Last Updated:** August 25, 2026  
**Estimated Build Time:** 56 hours over 4 weeks  

---

## 1. Executive Summary

**Apartment Hunter** is a 24/7 fraud-detection and deal-spotting system for SF Craigslist apartment listings. It automatically monitors Craigslist for new postings, runs a multi-signal fraud detection pipeline (reverse image search, OCR watermark detection, AI-generated-image classification), flags genuinely mispriced listings, and drafts personalized inquiry emails.

**Core value proposition:** Reduce manual vetting time from 2-3 hours/day to 15 minutes/day by eliminating 90% of scams automatically and highlighting deals worth pursuing.

**Constraints:**
- Single-platform focus (Craigslist only—no ToS violations, no scope creep to Zillow/Facebook)
- Budget constraint: ~$25/month operational cost
- Deployment: Runs unattended 24/7 on either $5 VPS or home Mac Mini
- Target: 100 listings/day ingestion, 10-20 clean listings identified daily, <3sec latency per listing

---

## 2. System Architecture Overview

### High-Level Data Flow

```
Flipify                  Go Service                    Outputs
(Craigslist              (Fraud Orchestration)         
API)        →  Poller   →  ┌─ Reverse Image Search
                            ├─ OCR (MLS Detection)      Telegram
                            ├─ AI Image Classifier      (Urgent Alerts)
                            ├─ Fraud Scoring            
                            └─ Market Comp Lookup   →   Prometheus
                                                        (Metrics)
                            ↓                           
                        SQLite                         Grafana
                        (Persistence)              →   (Dashboards)
                            
                            ↓                       Resend
                        Web UI                    (Digest Emails)
                        (Dashboard)
```

### Technology Stack & Rationale

| Component | Technology | Why |
|-----------|-----------|-----|
| **Core Service** | Go 1.21+ | Goroutines for parallel signal evaluation; static binary (~15MB) for low-memory VPS deployment |
| **ML Inference** | Python 3.11 + ONNX | TensorFlow/PyTorch ecosystem; gRPC sidecar for independent lifecycle |
| **Database** | SQLite | Filesystem-backed, zero-config, single-machine workload, easy to back up |
| **Monitoring** | Prometheus + Grafana | Industry standard for production systems; enables cost tracking + model health metrics |
| **Deployment** | Docker Compose | Single-host orchestration; avoids Kubernetes complexity at this scale |
| **Reverse Proxy** | Caddy | Auto HTTPS, simplicity for VPS deployment |
| **Remote Access (Home)** | Tailscale or Cloudflare Tunnel | Secure private access without port forwarding |

---

## 3. Detailed Architecture

### 3.1 Go Service (Core)

**Responsibilities:**
- Scheduled Flipify API poller (search new Craigslist listings every 2–5 minutes)
- Enrich each new listing once to retrieve its description and image set
- Orchestrate fraud signal evaluation (fan-out to parallel goroutines)
- Aggregate fraud scores
- Persist listings to SQLite
- Expose API for web UI
- Emit Prometheus metrics
- Call external APIs (Google Vision, BatchData, Resend, Telegram)

**Key Packages:**
```
cmd/
  └── apartment-hunter/        # Main binary
      └── main.go

internal/
  ├── ingestion/               # Flipify API polling + normalization
  │   ├── client.go             # v1 search, snapshot, and enrichment calls
  │   ├── poller.go             # Configurable scheduled polling loop
  │   └── types.go
  │
  ├── fraud/                   # Fraud detection orchestration
  │   ├── orchestrator.go       # Fan-out/fan-in logic
  │   ├── scorer.go            # Aggregate signals into score
  │   └── types.go
  │
  ├── signals/                 # Individual fraud detectors
  │   ├── reverse_image.go     # Google Vision integration
  │   ├── ocr.go               # MLS watermark detection
  │   ├── ai_classifier.go     # Call Python sidecar via gRPC
  │   └── market_comp.go       # BatchData comps lookup
  │
  ├── database/                # SQLite persistence
  │   ├── db.go
  │   ├── schema.go            # DDL
  │   └── queries.go
  │
  ├── api/                     # Web UI + external APIs
  │   ├── handlers.go
  │   └── routes.go
  │
  ├── metrics/                 # Prometheus instrumentation
  │   ├── prometheus.go
  │   └── cost_tracker.go
  │
  └── config/
      └── config.go            # Load from env + .env file

pkg/
  └── protos/                  # gRPC definitions (shared with Python)
      └── inference/
          ├── inference.proto
          └── inference_pb.go
```

**Critical Interfaces:**

```go
// orchestrator.go
type Orchestrator interface {
    EvaluateListing(ctx context.Context, listing *Listing) (*FraudResult, error)
}

// signals/reverse_image.go
type ReverseImageSignal interface {
    Evaluate(ctx context.Context, imageURL string) (*ReverseImageResult, error)
}

// fraud/scorer.go
type Scorer interface {
    Score(results *SignalResults) *FraudScore
}
```

**Goroutine Pattern (Core Parallelism):**

```go
// Fan-out/fan-in pattern for 3 signals evaluated in parallel
func (o *Orchestrator) EvaluateListing(ctx context.Context, l *Listing) {
    results := make(chan signal.Result, 3)
    
    go func() {
        result, err := o.reverseImage.Evaluate(ctx, l.Images)
        results <- result
    }()
    
    go func() {
        result, err := o.ocr.Evaluate(ctx, l.Images)
        results <- result
    }()
    
    go func() {
        result, err := o.aiClassifier.Evaluate(ctx, l.Images)
        results <- result
    }()
    
    // Collect all results
    for i := 0; i < 3; i++ {
        res := <-results
        // aggregate...
    }
}
```

**Metrics to Emit:**

```go
// Per-signal metrics
fraudDetectionLatency.WithLabelValues("reverse_image").Observe(duration)
fraudDetectionLatency.WithLabelValues("ocr").Observe(duration)
fraudDetectionLatency.WithLabelValues("ai_classifier").Observe(duration)

// Aggregate metrics
listingsProcessed.Inc()
fraudScore.Observe(score)
apiCallCost.WithLabelValues("google_vision").Add(cost)

// Error tracking
fraudDetectionErrors.WithLabelValues("google_vision", "not_found").Inc()
fraudDetectionErrors.WithLabelValues("grpc_timeout").Inc()
```

### 3.2 Python Sidecar (ML Inference)

**Responsibilities:**
- Load ONNX model (Sentry-Image or equivalent)
- Expose gRPC service for image classification
- Expose an HTTP `/metrics` endpoint on port 9091 for Prometheus
- Track model latency + error rates
- (Optional) periodic model re-evaluation on held-out test set

**Structure:**

```
inference-sidecar/
├── main.py                      # gRPC server startup
├── requirements.txt
├── model_loader.py              # Load ONNX from disk/HuggingFace
├── inference_service.py         # gRPC service implementation
└── model_eval.py                # Periodic re-eval (weekly)
```

**gRPC Interface (shared proto with Go):**

```protobuf
// protos/inference.proto
package inference;

service ImageClassifier {
    rpc ClassifyAIGenerated(ClassifyRequest) returns (ClassifyResponse);
}

message ClassifyRequest {
    string image_url = 1;
    string image_path = 2;  // Local path if already downloaded
}

message ClassifyResponse {
    float ai_score = 1;         // 0.0-1.0, higher = more likely AI
    float confidence = 2;       // Model confidence in the prediction
    int64 latency_ms = 3;       // Inference latency
}
```

**Model Choice:**
- Primary: Sentry-Image (NeurIPS 2023, Apache 2.0, self-hosted capable)
- Fallback: SPAI (CVPR 2025, spectral learning, generalizes better to unseen generators)
- Evaluation: Weekly re-eval on held-out test set; track precision/recall/F1 over time

**Cost Note:** Zero per-inference cost (self-hosted). Only infrastructure cost on shared VPS.

### 3.3 Database Schema (SQLite)

```sql
CREATE TABLE listings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    craigslist_id TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    price INTEGER NOT NULL,
    bedrooms REAL,
    bathrooms REAL,
    location TEXT,
    description TEXT,
    contact_email TEXT,
    contact_phone TEXT,
    posted_at DATETIME,
    craigslist_url TEXT,
    
    -- Fraud detection results
    fraud_score REAL,  -- 0.0-1.0
    fraud_flags JSON,  -- ['reverse_match_zillow', 'ai_generated', ...]
    fit_score REAL,  -- 0.0-1.0 against configurable price/location/bedroom preferences
    urgent_eligible BOOLEAN NOT NULL DEFAULT FALSE,
    fraud_evaluation_time INTEGER,  -- milliseconds to evaluate
    image_count INTEGER,
    images JSON,  -- [{'url': '...', 'ai_score': 0.75}, ...]
    
    -- Reverse image search results (cached)
    reverse_image_matches JSON,
    reverse_image_source TEXT,  -- 'zillow', 'redfin', 'stock_photo', 'none'
    
    -- Market intelligence (optional)
    neighborhood_avg_price INTEGER,
    price_percentile REAL,  -- Where listed price falls (0-100)
    comps_fetched_at DATETIME,
    
    -- User interactions
    status TEXT,  -- 'new', 'interested', 'contacted', 'toured', 'applied', 'passed'
    user_notes TEXT,
    contacted_at DATETIME,
    toured_at DATETIME,
    applied_at DATETIME,
    
    -- Metadata
    seen_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE fraud_eval_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    eval_date DATE,
    precision REAL,
    recall REAL,
    f1 REAL,
    false_positive_rate REAL,
    false_negative_rate REAL,
    total_test_samples INTEGER,
    model_version TEXT,  -- e.g., 'sentry-image-v1'
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE cost_ledger (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date DATE,
    service TEXT,  -- 'google_vision', 'flipify', 'infrastructure', 'batch_data', 'resend'
    cost REAL,
    unit_count INTEGER,  -- e.g., number of API calls
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE notification_deliveries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    listing_id INTEGER NOT NULL REFERENCES listings(id),
    channel TEXT NOT NULL,  -- 'telegram' or 'resend'
    notification_type TEXT NOT NULL,  -- 'urgent_alert' or 'digest'
    idempotency_key TEXT NOT NULL,
    provider_message_id TEXT,
    status TEXT NOT NULL,  -- 'pending', 'sent', 'failed'
    error_message TEXT,
    sent_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(listing_id, channel, notification_type),
    UNIQUE(idempotency_key)
);

CREATE INDEX idx_fraud_score ON listings(fraud_score);
CREATE INDEX idx_status ON listings(status);
CREATE INDEX idx_seen_at ON listings(seen_at);
```

### 3.4 Prometheus Metrics

**Dashboard 1: System Health**

```
# Throughput
apartment_hunter_listings_ingested_total{source="flipify"}
apartment_hunter_listings_processed_total{status="clean|fraud"}
apartment_hunter_listings_queue_depth

# Latency (per signal)
apartment_hunter_signal_latency_ms{signal="reverse_image",quantile="0.5|0.95|0.99"}
apartment_hunter_signal_latency_ms{signal="ocr",quantile="0.5|0.95|0.99"}
apartment_hunter_signal_latency_ms{signal="ai_classifier",quantile="0.5|0.95|0.99"}
apartment_hunter_total_evaluation_latency_ms{quantile="0.5|0.95|0.99"}

# Error rates
apartment_hunter_signal_errors_total{signal="reverse_image",error_type="api_error|timeout"}
apartment_hunter_signal_errors_total{signal="ocr",error_type="parse_error"}
apartment_hunter_api_errors_total{service="google_vision|flipify|resend|telegram",status_code="500|429|401"}

# Service health
apartment_hunter_service_uptime_seconds
apartment_hunter_last_flipify_poll_success_timestamp_seconds
```

**Dashboard 2: Model Health**

```
# Classification metrics (from fraud_eval_metrics table)
apartment_hunter_model_precision{model_version="sentry-image-v1"}
apartment_hunter_model_recall{model_version="sentry-image-v1"}
apartment_hunter_model_f1{model_version="sentry-image-v1"}
apartment_hunter_model_false_positive_rate{}
apartment_hunter_model_false_negative_rate{}

# Confidence distribution
apartment_hunter_fraud_score_histogram_bucket{le="0.1|0.2|...|1.0"}

# Drift detection
apartment_hunter_model_drift_score  # Computed weekly
```

**Dashboard 3: Cost Observability** ⭐

```
# Per-service costs (updated hourly from cost_ledger)
apartment_hunter_monthly_cost_total{service="flipify"}  # $14.99
apartment_hunter_monthly_cost_total{service="google_vision"}
apartment_hunter_monthly_cost_total{service="infrastructure"}
apartment_hunter_monthly_cost_total{service="batch_data"}
apartment_hunter_monthly_cost_total{service="resend"}

# Efficiency metrics
apartment_hunter_cost_per_listing{unit="processed|qualified|notified"}
apartment_hunter_cost_running_total  # Rolling 30-day sum

# Optimization impact
apartment_hunter_api_call_count{service="google_vision"}  # Spike = anomaly
```

### 3.5 Web UI

**Tech:** Python Flask (lightweight, simple, no JS framework overhead)

**Ownership Boundary:** Go is the only process allowed to open or write SQLite. Flask serves templates and calls the Go API at `API_BASE_URL`; every listing read and mutation, including status, notes, and notification history, goes through Go.

**Routes:**

```python
GET  /                              # Dashboard: overview + listing grid
GET  /api/listings                  # JSON: all clean listings, paginated
GET  /api/listings/<id>             # JSON: detail view
GET  /api/stats                     # JSON: today's stats (count, fraud rate, cost)
GET  /api/metrics                   # Redirect to Grafana (optional)
POST /api/listings/<id>/mark        # User interaction (contacted, toured, passed)
```

**Dashboard View:**
- Cards: Total seen today, clean listings, avg fraud score, cost spent today
- Table: List of clean listings (fraud_score, price, bedrooms, location, status)
- Color coding: Green (low fraud), Yellow (moderate fraud), Red (high fraud, shouldn't see this)
- Actions: Click → detail view, mark contacted/toured/passed

**Detail View:**
- Full listing info
- Fraud breakdown (reverse image matches, OCR results, AI score)
- Market comps (if available)
- Button: Open original listing
- User notes field

### 3.6 Deployment (Docker Compose)

**Services:**

```yaml
version: '3.8'
services:
  # Go core service
  fraud-service:
    build: .
    ports:
      - "8080:8080"      # API
      - "9090:9090"      # Prometheus metrics
    environment:
      - DATABASE_URL=sqlite:///data/apartments.db
      - GOOGLE_APPLICATION_CREDENTIALS=/run/secrets/gcp-service-account.json
      - FLIPIFY_API_KEY=${FLIPIFY_API_KEY}
      - BATCH_DATA_API_KEY=${BATCH_DATA_API_KEY}
      - RESEND_API_KEY=${RESEND_API_KEY}
      - NOTIFICATION_EMAIL_FROM=${NOTIFICATION_EMAIL_FROM}
      - NOTIFICATION_EMAIL_TO=${NOTIFICATION_EMAIL_TO}
    volumes:
      - ./data:/app/data          # Persist SQLite
      - ./secrets/gcp-service-account.json:/run/secrets/gcp-service-account.json:ro
    depends_on:
      - inference-sidecar
      - prometheus
    restart: unless-stopped

  # Python ML sidecar
  inference-sidecar:
    build: ./inference-sidecar
    expose:
      - "50051"         # gRPC, internal only
      - "9091"          # Prometheus metrics, internal only
    volumes:
      - ./models:/app/models
    restart: unless-stopped

  # Prometheus (scrapes Go /metrics endpoint)
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - ./data/prometheus:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
    restart: unless-stopped

  # Grafana (visualize Prometheus data)
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD}
      - GF_INSTALL_PLUGINS=grafana-piechart-panel
    volumes:
      - ./grafana/provisioning:/etc/grafana/provisioning
      - ./data/grafana:/var/lib/grafana
    depends_on:
      - prometheus
    restart: unless-stopped

  # Flask web UI
  web-ui:
    build: ./web
    ports:
      - "5000:5000"
    environment:
      - API_BASE_URL=http://fraud-service:8080
    depends_on:
      - fraud-service
    restart: unless-stopped

  # Optional: Caddy reverse proxy (VPS only, adds HTTPS)
  caddy:
    image: caddy:latest
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - ./data/caddy:/data
    depends_on:
      - grafana
      - web-ui
    restart: unless-stopped
```

**Configuration Files:**

`prometheus.yml`:
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'apartment-hunter'
    static_configs:
      - targets: ['fraud-service:9090']

  - job_name: 'inference-sidecar'
    static_configs:
      - targets: ['inference-sidecar:9091']
```

`Caddyfile` (VPS only):
```
grafana.yourdomain.com {
    reverse_proxy localhost:3000
    tls your-email@example.com
}

dashboard.yourdomain.com {
    reverse_proxy localhost:5000
    tls your-email@example.com
}
```

---

## 4. Implementation Plan

### Phase 1: Core Go Service (Weeks 1-2, ~24 hours)

**Objectives:**
- Poll Flipify's documented public API for Craigslist listings
- Enrich newly discovered listings once to retrieve descriptions and images
- Implement reverse image search + OCR signals
- Set up SQLite persistence
- Emit basic Prometheus metrics

**Milestones:**
1. **Day 1-2:** Project setup, database schema, config management
2. **Day 3-4:** Flipify API client, polling loop, and response normalization
3. **Day 5-7:** Reverse image search (Google Vision Web Detection)
4. **Day 8-10:** OCR + MLS watermark detection
5. **Day 11-12:** Fraud scoring, aggregate signals
6. **Day 13-14:** Prometheus metrics setup, test end-to-end

**Deliverables:**
- Running Go service that polls Flipify and persists newly discovered listings
- At least 50 test listings processed correctly
- Fraud scores computed for each
- Metrics exposed at `/metrics`

**Open Questions:**
- Confirm the API plan's actual credit allowance and rate limit before finalizing the polling interval.
- Should reverse image matches be cached (to reduce redundant Google Vision calls)?
- Threshold for "fraud score too high to process further"?

---

### Phase 2: ML Inference + Orchestration (Week 2, ~12 hours)

**Objectives:**
- Set up Python sidecar with ONNX inference
- Implement gRPC communication
- Integrate AI-generated-image classifier
- Parallel evaluation (goroutines + gRPC)

**Milestones:**
1. **Day 1-2:** Define and generate gRPC protos
2. **Day 3-4:** Python sidecar server + model loading
3. **Day 5-6:** Go client code (gRPC calls)
4. **Day 7:** Orchestrator fan-out/fan-in pattern

**Deliverables:**
- Python service running, callable via gRPC
- Model loads and infers in <500ms per image
- Go service calls sidecar successfully
- All three signals evaluated in parallel

---

### Phase 3: External APIs + Market Intel (Week 2-3, ~8 hours)

**Objectives:**
- Integrate BatchData for market comps (optional)
- Implement hybrid notifications (urgent Telegram + Resend digest email)

**Milestones:**
1. **Day 1-2:** BatchData comps lookup
2. **Day 3-4:** Telegram urgent alerts + Resend digest notifications

**Deliverables:**
- Market comps fetched (if opted in)
- Exceptional listings alerted immediately; all qualified listings included in scheduled email digests

---

### Phase 4: Observability (Weeks 3-4, ~12 hours)

**Objectives:**
- Prometheus dashboards (system health)
- Model health tracking (precision/recall)
- Cost observability dashboard
- Weekly model re-evaluation

**Milestones:**
1. **Day 1-3:** System health dashboard (throughput, latency, errors)
2. **Day 4-5:** Model health tracking + weekly eval
3. **Day 6-7:** Cost ledger + cost dashboard
4. **Day 8:** Grafana provisioning, multi-dashboard setup

**Deliverables:**
- Three Grafana dashboards live
- Cost tracking working (daily ledger updates)
- Model eval metrics captured weekly
- Drift detection logic implemented

---

### Phase 5: Web UI + Deployment (Weeks 4, ~12 hours)

**Objectives:**
- Flask web UI (dashboard and listing detail)
- Docker Compose setup
- Deployment docs (VPS + Mac Mini)

**Milestones:**
1. **Day 1-2:** Flask routes + templates
2. **Day 3-4:** User interaction tracking (database updates)
3. **Day 5-6:** Docker Compose, local testing
4. **Day 7-8:** VPS deployment + HTTPS (Caddy)
5. **Day 9:** Mac Mini setup (Tailscale/Cloudflare Tunnel)

**Deliverables:**
- Web UI fully functional
- Deployable via Docker Compose
- Running on actual VPS or home hardware
- Documentation for future deployments

---

### Phase 6: Optimization + Polish (Week 4, ~8 hours)

**Objectives:**
- Performance tuning (latency targets)
- Error handling + resilience
- Edge case testing
- Documentation

**Milestones:**
1. **Day 1-2:** Latency optimization (target <3sec/listing)
2. **Day 3:** Circuit breakers for external API failures
3. **Day 4:** Edge case testing (malformed listings, image failures)
4. **Day 5:** README, design doc, cost breakdown doc

**Deliverables:**
- System consistently processes listings in <3sec
- Graceful degradation when APIs are down
- Comprehensive README + design documentation

---

## 5. External APIs & Integrations

### Flipify (Craigslist Monitoring)

**What:** The approved Craigslist ingestion provider. The documented public v1 API uses authenticated requests rather than webhook delivery.

**Integration:**
- `GET /listings/search` with `market=Craigslist`, a search query, and SF latitude/longitude returns normalized basic listing fields. Poll on a configurable 2–5 minute interval.
- Persist Flipify's opaque listing ID and deduplicate before any enrichment. Do not parse or construct this ID.
- `POST /listings/{listing_id}/enrich` once for each new listing to request `description`, `images`, `location`, and `attributes`.
- `GET /listings/{listing_id}` is available for a no-refresh basic snapshot.
- Initial configured charges are one credit per search page (including empty results), zero for basic snapshots, and one credit per enrichment. The account's active configuration and response headers are authoritative.

**Preflight Before Enabling the Poller:**
- Query `GET /account/credits` and `GET /account/usage` with the API key.
- Record the included API-credit allowance, rate limits, and actual per-operation cost in the cost ledger.
- Set `FLIPIFY_POLL_INTERVAL` to `5m` initially; only reduce it toward `2m` after confirming the account allowance supports the observed usage.

**Reliability Note:** Flipify owns marketplace collection. The Mac Mini owns polling, idempotency, API retries/backoff, and persistence. If Flipify is unavailable, retain the last successful poll timestamp and surface a degraded-ingestion alert.

### Google Cloud Vision API

**Services Used:**
1. **Web Detection** ($3.50/1k images): Reverse image search
2. **Text Detection (OCR)** ($1.50/1k images): MLS watermark detection

**MVP Usage Guardrail:**
- Cap each feature at fewer than 1,000 images/month: the first 1,000 monthly units of Web Detection and Text Detection are free separately.
- Start with one primary image per candidate listing; cache image hashes and never evaluate the same image twice.
- Track both Vision feature-unit counts and the skipped-due-to-budget count in Prometheus and the cost ledger.

**Setup:**
```bash
# Create GCP project, enable Vision API, create service account
gcloud auth application-default login
export GOOGLE_APPLICATION_CREDENTIALS=path/to/service-account-key.json
```

### BatchData API (Optional—Market Comps)

**Service:** Rental comps lookup ($0.01/call)

**Usage:**
- 50-100 comps lookups/month
- Cost: ~$0.50-1.00/month

**Setup:**
```bash
export BATCH_DATA_API_KEY=...
```

**Note:** This is optional. If budget-conscious, skip it (comps are "nice-to-have").

### Resend (Digest Email)

**Purpose:** Send one scheduled, transactional digest covering all newly qualified listings.

**Setup:**
1. Verify a sending domain in Resend and create a sending-only API key.
2. Configure a sender and recipient address.
3. Compose one HTML/text digest every 12 hours (configurable) from qualified listings not yet included in a digest.

```bash
export RESEND_API_KEY=re_...
export NOTIFICATION_EMAIL_FROM=alerts@yourdomain.com
export NOTIFICATION_EMAIL_TO=you@example.com
export RESEND_DIGEST_INTERVAL=12h
```

**Digest Content:** Title, price, location, risk summary, score, and the original listing URL for each included listing. Do not include contact details or full descriptions. Use an idempotency key derived from the digest window, then record each included listing in `notification_deliveries` only after Resend accepts the digest.

### Telegram Bot (Urgent Alerts)

**Purpose:** Immediately alert on exceptional listings that meet the strict urgent-notification rule: low fraud risk, a strong deal signal, and no hard fraud flags. Telegram is not used for normal qualified listings.

**Setup:**
1. Create a bot through BotFather and obtain the bot token plus your chat ID.
2. Send an idempotent message for each urgent listing.
3. Include all urgent listings in the next Resend digest as well, so email remains the complete review record.

```bash
export TELEGRAM_BOT_TOKEN=...
export TELEGRAM_CHAT_ID=...
```

**Alert Content:** Title, price, location, concise reason it is urgent, fraud-risk score, and original listing URL. Do not include contact details or full descriptions.

**Urgent-Notification Rule:** A listing receives a Telegram alert only when all conditions hold: it is newly discovered, `fraud_score <= 0.20`, `fit_score >= 0.80`, no hard fraud flag is present, and no prior `urgent_alert` delivery exists. `fit_score` is deterministic and based on configurable price, location, and bedroom preferences; it is not an AI judgment. All other qualified listings wait for the next Resend digest.

---

## 6. Cost Breakdown

**Monthly Operating Cost:**

| Service | Cost | Notes |
|---------|------|-------|
| Flipify | Plan-dependent | Verify actual API allowance and charges through `/account/usage` |
| Google Vision | $0.00 target | Keep each feature below 1,000 image units/month |
| Resend | $0.00 | Transactional alerts within the free plan |
| Telegram Bot API | $0.00 target | Urgent personal alerts only |
| Infrastructure (Mac Mini) | Existing hardware | Small incremental electricity cost only |
| BatchData (optional) | $0.50 | Market comps (can skip) |
| **Total** | **Pending Flipify plan verification** | Record actual cost in the ledger during the first month |

**Cost per Listing:**
- Compute from the cost ledger after the first month of production-like operation.
- Report separately for processed listings, qualified listings, and notification emails.

---

## 7. Testing Strategy

### Unit Tests (Go)

```go
// Test fraud scoring logic
func TestScorer_HighRiskImages(t *testing.T) {
    s := NewScorer()
    results := &SignalResults{
        ReverseImage: &ReverseImageResult{Risk: "high"},
        OCR: &OCRResult{MLS: true},
    }
    score := s.Score(results)
    if score.Score < 0.8 {
        t.Fail()
    }
}

// Test database persistence
func TestDB_InsertAndQuery(t *testing.T) {
    db := setupTestDB()
    listing := &Listing{ID: "123", Title: "Test"}
    err := db.Insert(listing)
    retrieved, _ := db.Get("123")
    if retrieved.Title != "Test" {
        t.Fail()
    }
}
```

### Integration Tests

```go
// End-to-end test: Flipify poll + enrichment → fraud evaluation → database write
func TestEndToEnd_ProcessListing(t *testing.T) {
    // Start Go service in test mode
    // Mock Flipify search and enrichment responses
    // Verify listing appears in database with fraud score
    // Verify Prometheus metrics incremented
}
```

### Model Evaluation (Python)

```python
# Weekly evaluation on held-out test set
def evaluate_model_weekly():
    model = load_model()
    test_set = load_test_images()  # 100-200 labeled images
    
    predictions = model.predict(test_set)
    metrics = compute_metrics(predictions, test_set.labels)
    
    # precision, recall, f1, fpr, fnr
    log_to_prometheus(metrics)
    log_to_database(metrics)
```

---

## 8. Deployment Targets

### Option A: Cloud VPS ($5/month)

**Recommended for:** Portfolio piece (public URL, always-on)

**Setup:**
1. DigitalOcean droplet: Ubuntu 24, 1 CPU, 512MB RAM
2. SSH in, install Docker + Docker Compose
3. Clone repo, create `.env` file
4. `docker-compose up -d`
5. Caddy handles HTTPS automatically

**Access:**
- Grafana: `https://grafana.yourdomain.com`
- Web UI: `https://dashboard.yourdomain.com`

### Option B: Home Mac Mini ($0 ongoing)

**Recommended for:** Personal use + learning

**Setup:**
1. Install Docker Desktop
2. Clone repo, create `.env` file
3. `docker-compose up -d`
4. (Optional) Tailscale for private remote access
5. (Optional) Cloudflare Tunnel for shareable demo URL

**Access:**
- Grafana: `http://localhost:3000` (local only, or via Tailscale)
- Web UI: `http://localhost:5000`

---

## 9. Known Constraints & Assumptions

**Constraints:**
1. **Craigslist-only:** No Zillow, Facebook Marketplace (scope, ToS)
2. **SF Bay Area:** Assumes Flipify search is pre-configured for SF apartments
3. **Unattended operation:** Designed to run 24/7 without manual intervention
4. **Single machine:** No horizontal scaling (not needed at 100 listings/day)
5. **Budget:** Must stay under $30/month operational cost

**Assumptions:**
1. Flipify public API access is enabled for the selected account and its allowance supports the configured polling interval.
2. Google Vision API is available (degraded performance during outages)
3. SQLite is sufficient for this workload (true at 100 listings/day)
4. Research-backed signals are risk indicators, not ground truth; outcomes require independent manual verification.

**Known Limitations:**
1. AI image detection degrades against newer generators (requires periodic model updates)
2. False positives possible (some clean listings might score high)
3. False negatives possible (sophisticated scams might slip through)
4. Market comps are point-in-time (don't track trends)

**Deferred from MVP:** AI-generated landlord-inquiry drafting (including Claude) is intentionally out of scope. Reconsider it only after the core discovery, review, and notification workflow is dependable.

---

## 10. Success Criteria (MVP Definition)

**Technical:**
- ✅ System processes 100 listings/day
- ✅ Latency: <3 seconds per listing
- ✅ Fraud detection pipeline runs all three signals in parallel
- ✅ Fraud scores computed and stored
- ✅ Prometheus metrics exposed and dashboards live
- ✅ Cost tracked daily (updated in Grafana)
- ✅ Deployed and running 24/7
- ✅ Web UI functional (view listings and mark interactions)

**Portfolio:**
- ✅ Code is clean, well-documented, production-ready
- ✅ GitHub repo with comprehensive README
- ✅ Design document explaining architecture choices
- ✅ Cost breakdown document showing financial thinking
- ✅ Deployable via Docker Compose (one command)
- ✅ Demonstrates: systems thinking, API integration, ML inference, observability, cost consciousness

**User Feedback (Self):**
- ✅ System actually reduces time spent apartment hunting
- ✅ Fraud detection catches 80%+ of obvious scams
- ✅ Dashboard is easy to check (takes <2 min/day to monitor)

---

## 11. Open Questions & Decisions

**Before Starting:**

1. **Craigslist search configuration:** Which Flipify searches to monitor? (e.g., "SF Bay 1BR $2000-$3000" + "SF Bay 2BR $3000-$4000")
   - Recommendation: Start with one search, expand later

2. **Market comps—include or skip?** BatchData adds $0.50-1/month but is optional.
   - Recommendation: Skip in MVP, add in V1.1 if time allows

3. **Deployment preference:** VPS or home Mac Mini?
   - Portfolio angle: VPS (public URL)
   - Learning angle: Mac Mini (understand home deployment)
   - Recommendation: Start with Mac Mini, migrate to VPS once working

4. **Notification channels:** Telegram for urgent alerts and Resend for scheduled digest email.
   - Decision: Immediate alerts are reserved for exceptional listings; email provides the complete, searchable review trail.

5. **What API allowance and rate limit apply to the Flipify key?**
   - Recommendation: Run `/account/credits` and `/account/usage` during setup, begin at a 5-minute polling interval, and revise only with observed usage.

**During Implementation:**

1. **Model re-evaluation frequency:** Weekly, monthly, or on-demand?
   - Recommendation: Weekly (catch drift early)

2. **How many images per listing to evaluate?** (vs. Vision budget)
   - Recommendation: Start with one primary image per candidate and remain below 1,000 monthly units per Vision feature; expand only after reviewing value and cost.

3. **Notification thresholds:** Qualified listings use `fraud_score < 0.50`; Telegram urgency additionally requires `fraud_score <= 0.20`, `fit_score >= 0.80`, and no hard fraud flags.

---

## 12. Documentation to Write

**Before shipping:**
1. **README.md** — Quick start, project overview, build instructions
2. **ARCHITECTURE.md** — System design, component responsibilities, data flow
3. **DEPLOYMENT.md** — VPS setup, Mac Mini setup, Docker Compose reference
4. **COSTS.md** — Cost breakdown, optimization strategies, monthly trends
5. **API.md** — Web UI endpoints, Flipify polling contract, Prometheus metrics list
6. **DESIGN_DECISIONS.md** — Why Go, why Prometheus, why SQLite, etc.

---

## 13. Portfolio Narrative (For Interviews)

**Problem Statement:**
*"Finding apartments in SF involves manually reviewing hundreds of Craigslist listings, 85% of which are scams. You reverse-image-search photos to catch stolen images, scan for MLS watermarks, check prices against comps—all tedious manual work. I built a system to automate the vetting layer so I only see listings worth pursuing."*

**Solution:**
*"Apartment Hunter monitors Craigslist 24/7, runs a fraud detection pipeline (reverse image search, OCR, AI image classification), and flags genuinely mispriced listings. Everything runs unattended on a $5 VPS."*

**Technical Highlights:**
- **Go + goroutines:** Parallel signal evaluation under tight latency budgets
- **Prometheus + Grafana:** Real-time observability of system health, model quality, and operational cost
- **Cost tracking:** Shows I think like an operator—I know exactly what this costs to run
- **Deployment:** Runnable via Docker Compose on any machine (VPS, Mac Mini, laptop)

**What This Shows:**
- Systems design (coordinating multiple services)
- Production mindset (observability, cost consciousness, operational clarity)
- API integration (Google Vision, Flipify, BatchData, Resend, Telegram)
- ML model usage (self-hosted inference sidecar)
- Full-stack thinking (Go backend + Python sidecar + Flask UI + Prometheus/Grafana)

---

## 14. Next Steps

1. **Review & Agree:** Does this handoff align with your vision?
2. **Clarify:** Answer any open questions (deployment choice, search config, etc.)
3. **Setup:** Create GitHub repo, set up local development environment
4. **Start:** Begin Phase 1 (Go service + database schema)

---

**Prepared for:** Implementation  
**Status:** Ready  
**Questions?** See Section 11 (Open Questions)
