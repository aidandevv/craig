# Apartment Hunter: Implementation Checklist

## Pre-Implementation Setup (1-2 hours)

- [ ] Create GitHub repo (github.com/aidandevaney/apartment-hunter)
- [ ] Set up local dev environment (Go 1.21+, Python 3.11+, Docker, Docker Compose)
- [ ] Create `.env.example` template with required keys:
  - [ ] GOOGLE_APPLICATION_CREDENTIALS=/absolute/path/to/service-account.json
  - [ ] FLIPIFY_API_KEY
  - [ ] FLIPIFY_POLL_INTERVAL=5m
  - [ ] BATCH_DATA_API_KEY (optional)
  - [ ] RESEND_API_KEY
  - [ ] NOTIFICATION_EMAIL_FROM
  - [ ] NOTIFICATION_EMAIL_TO
  - [ ] RESEND_DIGEST_INTERVAL=12h
  - [ ] TELEGRAM_BOT_TOKEN
  - [ ] TELEGRAM_CHAT_ID
  - [ ] QUALIFIED_MAX_FRAUD_SCORE=0.50
  - [ ] URGENT_MAX_FRAUD_SCORE=0.20
  - [ ] URGENT_MIN_FIT_SCORE=0.80
  - [ ] DATABASE_URL=sqlite:///data/apartments.db
- [ ] Set up Flipify account and obtain a public API key with Craigslist access
- [ ] Confirm Flipify API allowance and rate limits using `/account/credits` and `/account/usage`
- [ ] Create GCP project, enable Vision API, download service account key
- [ ] Verify a Resend sending domain and create a sending-only API key
- [ ] Create Telegram bot via BotFather; capture bot token and chat ID
- [ ] Get BatchData API key (optional)

## Phase 1: Go Core Service (Weeks 1-2)

### Database & Configuration
- [ ] Design SQLite schema (listings table + fraud_eval_metrics + cost_ledger)
- [ ] Make Go the sole SQLite owner; Flask accesses listings and writes only through the Go API
- [ ] Implement database initialization (schema DDL, migrations)
- [ ] Implement config loading from `.env` file (viper or similar)
- [ ] Set up structured logging (slog or zap)

### Flipify API Ingestion
- [ ] Create HTTP server (net/http or Gin)
- [ ] Implement authenticated Flipify v1 client (`/listings/search`, `/{id}`, `/{id}/enrich`)
- [ ] Implement scheduled 5-minute poller (configurable to 2–5 minutes after allowance verification)
- [ ] Query Craigslist with required latitude/longitude, search filters, and pagination handling
- [ ] Normalize basic listing fields and persist Flipify's opaque listing ID
- [ ] Implement duplicate detection (skip IDs already seen)
- [ ] Enrich each new listing once for description, images, location, and attributes
- [ ] Add API retry/backoff, rate-limit handling, and last-successful-poll logging

### Google Vision Integration
- [ ] Integrate Google Cloud Vision SDK
- [ ] Enforce an under-1,000-images/month cap separately for Web Detection and OCR
- [ ] Start with one primary image per candidate; skip and record later images when the cap is reached
- [ ] Implement reverse image search (WebDetection API)
- [ ] Parse results, determine risk level (zillow/redfin match = high risk, etc.)
- [ ] Implement OCR for MLS watermark detection (TextDetection API)
- [ ] Cache reverse image search results (avoid duplicate API calls)
- [ ] Add error handling (API unavailable, invalid images, etc.)

### Fraud Scoring
- [ ] Define fraud scoring algorithm (how to weight signals)
- [ ] Implement orchestrator (fan-out/fan-in goroutines for parallel signals)
- [ ] Implement scorer (aggregate signals → fraud_score 0.0-1.0)
- [ ] Store results in database (fraud_score, fraud_flags, images JSON)
- [ ] Implement filtering logic (threshold for "clean" vs "fraud")
- [ ] Implement deterministic `fit_score` from configured price, location, and bedroom preferences

### Prometheus Setup
- [ ] Add prometheus/client_golang dependency
- [ ] Create metrics (see Section 5 of handoff)
- [ ] Implement `/metrics` endpoint
- [ ] Instrument goroutine latency (per signal)
- [ ] Instrument error rates
- [ ] Instrument listing counts

### Testing & Validation
- [ ] Unit tests for fraud scoring logic
- [ ] Unit tests for database operations
- [ ] End-to-end test: mock Flipify search + enrichment responses → database write
- [ ] Manual test: send 50 test listings, verify scores are computed
- [ ] Verify metrics are exposed at `/metrics`

**Go to Phase 2 when:**
- ✅ Service polls Flipify continuously and records its last successful poll
- ✅ 50+ test listings processed successfully
- ✅ Fraud scores computed and stored
- ✅ Metrics exposed at /metrics

---

## Phase 2: ML Inference Sidecar (Week 2)

### Proto Definitions
- [ ] Define `inference.proto` (ImageClassifier service, ClassifyRequest/Response)
- [ ] Generate Go code (protoc)
- [ ] Generate Python code (protoc)

### Python Service
- [ ] Create Python project structure (main.py, requirements.txt, model_loader.py)
- [ ] Implement model loader (load Sentry-Image from HuggingFace or local)
- [ ] Implement gRPC server (grpcio, grpcio-tools)
- [ ] Implement inference logic (image → float score)
- [ ] Add latency tracking (milliseconds per inference)
- [ ] Handle image loading (URL or local path)
- [ ] Add error handling (model not found, inference failure)
- [ ] Implement health check gRPC method (for Docker Compose readiness)

### Go gRPC Client
- [ ] Add gRPC client library (google.golang.org/grpc)
- [ ] Implement gRPC client for inference sidecar
- [ ] Call classifier in orchestrator (parallel with other signals)
- [ ] Handle gRPC errors (timeout, unavailable, etc.)
- [ ] Add circuit breaker (fail gracefully if sidecar is down)

### Integration
- [ ] Test Python sidecar in isolation (send test image, get score)
- [ ] Test Go → Python communication (gRPC call)
- [ ] Verify parallel evaluation (all three signals in parallel)

**Go to Phase 3 when:**
- ✅ Python sidecar runs and serves gRPC requests
- ✅ Inference latency is <500ms per image
- ✅ Go service calls sidecar successfully
- ✅ All three signals evaluated in parallel

---

## Phase 3: External APIs & Notifications (Week 2-3)

### BatchData Integration (Optional)
- [ ] Integrate BatchData API (if opted in)
- [ ] Query comps for neighborhood
- [ ] Store in database (neighborhood_avg_price, price_percentile)
- [ ] Handle errors gracefully (skip if unavailable)

### Resend Digest Email
- [ ] Implement Resend transactional-email client
- [ ] Compose one 12-hour digest (configurable) from newly qualified listings
- [ ] Format digest items: title, price, location, fraud risk, score, and original URL
- [ ] Use a digest-window idempotency key and retry uncertain sends with the same key
- [ ] Record each included listing to prevent it from appearing in a later digest
- [ ] Store provider message ID and delivery-attempt outcome
- [ ] Handle Resend API errors without blocking listing persistence

### Telegram Urgent Alerts
- [ ] Implement Telegram Bot API client
- [ ] Enforce urgent rule: new listing, `fraud_score <= 0.20`, `fit_score >= 0.80`, no hard flag, and no prior urgent delivery
- [ ] Send one idempotent alert for each urgent listing
- [ ] Store provider message ID and delivery-attempt outcome separately from digest delivery
- [ ] Handle Telegram API errors without blocking listing persistence

### Testing
- [ ] Test Resend digest (verify each new qualified listing appears once and contains no contact details)
- [ ] Test Telegram urgent alert (verify only urgent listings alert immediately and no duplicates are sent)
- [ ] Test BatchData comps (verify prices are reasonable)

**Go to Phase 4 when:**
- ✅ Urgent listings reach Telegram without duplicates; qualified listings reach the next Resend digest once
- ✅ (Optional) Market comps fetched

---

## Phase 4: Observability & Metrics (Weeks 3-4)

### Cost Tracking
- [ ] Implement cost ledger (cost_ledger table: date, service, cost, unit_count)
- [ ] Track Google Vision costs (per API call)
- [ ] Track infrastructure cost ($5/mo manually, or calculate)
- [ ] Track Flipify cost (manual, $14.99/mo)
- [ ] Implement daily summary (total cost so far this month)

### Prometheus Dashboards
- [ ] Create Grafana data source pointing to Prometheus
- [ ] Dashboard 1: System Health
  - [ ] Throughput graph (listings ingested, processed, clean/fraud)
  - [ ] Latency graph (per signal: p50, p95, p99)
  - [ ] Error rate graph (failures by service)
  - [ ] Queue depth
  - [ ] Uptime
- [ ] Dashboard 2: Model Health
  - [ ] Precision/Recall/F1 (from weekly eval)
  - [ ] Fraud score distribution histogram
  - [ ] False positive/negative rates
  - [ ] Drift score
- [ ] Dashboard 3: Cost Observability ⭐
  - [ ] Cost per service (Flipify, Google Vision, Resend, Telegram, infra)
  - [ ] Total monthly cost (rolling 30-day)
  - [ ] Cost per listing, cost per clean listing
  - [ ] Trend over time (are costs increasing? optimizations working?)

### Model Evaluation
- [ ] Create held-out test set (100-200 labeled images)
- [ ] Implement weekly evaluation (precision, recall, F1, FPR, FNR)
- [ ] Log metrics to Prometheus (for dashboard)
- [ ] Log metrics to database (for historical tracking)
- [ ] Implement drift detection (compare week-to-week)

### Prometheus Configuration
- [ ] Create prometheus.yml (scrape intervals, targets)
- [ ] Test local Prometheus scraping
- [ ] Verify all metrics appear in Prometheus UI

**Go to Phase 5 when:**
- ✅ Grafana accessible and dashboards live
- ✅ Cost tracking working (daily ledger updates)
- ✅ Model eval metrics appearing in dashboard
- ✅ Drift detection implemented

---

## Phase 5: Web UI & Deployment (Week 4)

### Flask Web UI
- [ ] Create Flask project structure (app.py, templates/, static/)
- [ ] Configure `API_BASE_URL` and ensure Flask never opens the SQLite database
- [ ] Implement `/` route (dashboard with stats + listing grid)
- [ ] Implement `/api/listings` (JSON endpoint, paginated)
- [ ] Implement `/api/listings/<id>` (detail view)
- [ ] Implement `/api/stats` (today's stats: count, fraud rate, cost)
- [ ] Implement `POST /api/listings/<id>/mark` (track user interactions)
- [ ] Add basic CSS/styling (Tailwind or minimal CSS)
- [ ] Add client-side interactivity (mark contacted, toured, passed)

### Docker Compose
- [ ] Create Dockerfile for Go service
- [ ] Create Dockerfile for Python sidecar
- [ ] Create Dockerfile for Flask UI
- [ ] Create docker-compose.yml (all services)
- [ ] Test locally: `docker-compose up`, verify all services start
- [ ] Verify inter-service communication (Go → Python gRPC, Flask → Go API)

### VPS Deployment (Optional but Recommended)
- [ ] Create DigitalOcean droplet (Ubuntu 24, 1CPU, 512MB)
- [ ] SSH in, install Docker + Docker Compose
- [ ] Clone repo, set up `.env` file with API keys
- [ ] Run `docker-compose up -d`
- [ ] Verify services running: curl http://localhost:8080/metrics
- [ ] Create DNS entries for your domain (grafana.yourdomain.com, dashboard.yourdomain.com)
- [ ] Set up Caddy (reverse proxy with auto HTTPS)
- [ ] Test: visit https://grafana.yourdomain.com, https://dashboard.yourdomain.com

### Mac Mini Deployment (Optional)
- [ ] Install Docker Desktop
- [ ] Clone repo, set up `.env` file
- [ ] Run `docker-compose up -d`
- [ ] (Optional) Install Tailscale for private remote access
- [ ] (Optional) Set up Cloudflare Tunnel for shareable URL

### Testing & Validation
- [ ] Local end-to-end test: mock Flipify poll + enrichment → verify in web UI
- [ ] Verify all three dashboards load
- [ ] Verify user can mark listing as contacted/toured/passed
- [ ] Monitor logs: `docker-compose logs -f`

**Go to Phase 6 when:**
- ✅ Web UI fully functional locally
- ✅ Docker Compose works on your machine
- ✅ (Optional) Deployed to VPS or Mac Mini
- ✅ Public URL works (if VPS)

---

## Phase 6: Polish & Documentation (Week 4)

### Performance Optimization
- [ ] Latency profiling: identify bottlenecks
- [ ] Target: <3 seconds per listing end-to-end
- [ ] Optimize image download parallelism
- [ ] Optimize database queries (add indexes)
- [ ] Batch API calls where possible (reduce individual calls)

### Resilience & Error Handling
- [ ] Circuit breakers for external services (Google Vision, Flipify, Resend, Telegram)
- [ ] Graceful degradation (e.g., skip comps if BatchData unavailable)
- [ ] Retry logic with exponential backoff
- [ ] Handle malformed images, missing fields, network timeouts
- [ ] Comprehensive error logging

### Edge Cases
- [ ] Duplicate listings (same ID processed twice)
- [ ] Empty image arrays
- [ ] Invalid image URLs (404, etc.)
- [ ] Listings with no text (title, description empty)
- [ ] API failures (Google Vision, Flipify, Resend, or Telegram unavailable)
- [ ] Database lock contention (SQLite under concurrent writes)

### Documentation
- [ ] Write README.md (problem, solution, quick start, build instructions)
- [ ] Write ARCHITECTURE.md (system design, components, data flow)
- [ ] Write DEPLOYMENT.md (VPS setup, Mac Mini setup, Caddy config)
- [ ] Write COSTS.md (cost breakdown, monthly estimates, optimization strategies)
- [ ] Write API.md (web UI endpoints, Flipify polling contract, Prometheus metrics)
- [ ] Write DESIGN_DECISIONS.md (why Go, why Prometheus, why SQLite, trade-offs)

### Code Quality
- [ ] Add unit tests (minimum 50% coverage)
- [ ] Add integration tests (end-to-end workflow)
- [ ] Code review checklist (readability, error handling, style)
- [ ] Linting (golangci-lint for Go, black/flake8 for Python)
- [ ] Documentation comments (explain non-obvious logic)

### GitHub Repo
- [ ] Add .gitignore (API keys, data/, *.db)
- [ ] Add LICENSE (MIT or Apache)
- [ ] Add CONTRIBUTING.md (if sharing)
- [ ] Create GitHub issues for future work (V1.1, V2, etc.)
- [ ] Add GitHub Actions for CI/CD (optional but good practice)

**MVP Complete when:**
- ✅ All code documented
- ✅ README is clear and complete
- ✅ Repo is clean and ready to share
- ✅ Deployment docs work for someone else
- ✅ System runs 24/7 without manual intervention
- ✅ Cost tracking shows actual numbers

---

## Testing & Validation (Ongoing)

### Daily During Implementation
- [ ] Verify new code doesn't break existing functionality
- [ ] Check error logs: `docker-compose logs fraud-service`
- [ ] Monitor Prometheus: are all metrics appearing?
- [ ] Test Telegram urgency gating and Resend digest deduplication

### Weekly
- [ ] Run model evaluation (precision/recall on test set)
- [ ] Check cost ledger (is cost tracking working? does it match estimates?)
- [ ] Review dashboard (do system health metrics look normal?)
- [ ] Look for anomalies (latency spikes, error rate changes, drift)

### Before Shipping (Phase 6 Exit Criteria)
- [ ] System has processed 1,000+ listings without errors
- [ ] Average latency is <3 seconds per listing
- [ ] Fraud detection accuracy is >85% (based on manual spot checks)
- [ ] Cost is within budget (<$30/month)
- [ ] Dashboards are live and updating in real-time
- [ ] Documentation is complete and accurate
- [ ] Deployment is reproducible (anyone can `docker-compose up`)

---

## Deployment Checklist (Before Going Live)

- [ ] API keys are stored securely (never in code, only in .env)
- [ ] .env file is in .gitignore (prevent accidental commit)
- [ ] Database backups are configured (SQLite to /data)
- [ ] Monitoring alerts are set (optional: Discord/email if service down)
- [ ] Caddy/HTTPS is working (if VPS deployment)
- [ ] Grafana password is set (not default)
- [ ] Cron job exists for daily cost ledger updates (optional but good)
- [ ] README includes troubleshooting section
- [ ] You can deploy a fresh instance from scratch in <30 minutes

---

## Post-Deployment (Ongoing Operations)

### Weekly
- [ ] Check Grafana dashboards (system health, cost)
- [ ] Review model evaluation metrics (any drift?)
- [ ] Look at top fraud flags (are scam patterns changing?)

### Monthly
- [ ] Review cost trends (is budget still accurate?)
- [ ] Update model if drift detected (retrain or pick new model)
- [ ] Backup database (SQLite to external storage)

### As Needed
- [ ] Fix broken selectors if Craigslist HTML changes
- [ ] Update API integrations if services change
- [ ] Optimize slow queries if performance degrades

---

## Success Metrics (How to Know It's Working)

**System Health:**
- Listings processed per day: 100+ ✅
- Avg latency per listing: <3 seconds ✅
- Error rate: <1% ✅
- Uptime: >99% ✅

**Fraud Detection:**
- Fraud accuracy: >85% ✅
- False positive rate: <10% ✅
- False negative rate: <15% ✅
- Signal diversity: all 3 signals active ✅

**Cost:**
- Monthly cost: <$30 ✅
- Cost per listing: <$0.01 ✅
- Breakdown visible in Grafana ✅

**User Experience:**
- Time to review listings: <10 min/day ✅
- Notifications arrive within 1 min of posting ✅

**Portfolio:**
- Code is clean and documented ✅
- Deployable in one command ✅
- Shows systems thinking ✅
- Interviews: can explain architecture confidently ✅

## Deferred from MVP

- [ ] Re-evaluate AI-generated landlord-inquiry drafts only after the core discovery, review, and notification workflow is dependable. Do not add an Anthropic/Claude credential, endpoint, or cost tracking before then.

---

## Estimated Timeline

```
Week 1:    Phases 1a (Core Go service, database, Flipify polling)
Week 2:    Phases 1b (Fraud signals), 2 (ML sidecar)
Week 3:    Phases 3 (External APIs), 4a (Cost tracking)
Week 4:    Phases 4b (Dashboards), 5 (Web UI + deployment), 6 (Polish)

Total: 56 hours over 4 weeks
Pace: ~1 hour/day (sustainable)
```

---

## Remember

- Don't over-engineer. Scope creep kills projects.
- Craigslist-only is intentional. Avoid adding Zillow/Facebook.
- Observability from day one (no last-minute metrics addition).
- Cost consciousness is a feature (not a bug to hide).
- Deployment matters (it's part of the portfolio).
- Documentation is part of the code (not an afterthought).

Good luck! 🚀
