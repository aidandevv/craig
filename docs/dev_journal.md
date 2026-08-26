# Engineering Development Journal
> Chronological log of architecture decisions, implementation pivots, constraint discoveries, verification results, and engineering milestones.

---

### [CP-PIVOT] | 2026-08-25: Implement Flipify API polling with a Vision budget cap

**Summary:** The handoff and checklist now replace the assumed Flipify webhook with its documented v1 search-and-enrichment API. The initial poll interval is five minutes, configurable toward two minutes only after the account allowance is verified; Google Vision evaluation begins with one image per candidate and a separate under-1,000-unit cap for each feature.

**Files/Modules Affected:** `files/apartment-hunter-handoff.md`, `files/IMPLEMENTATION_CHECKLIST.md`.

**Key Trade-off:** Scheduled polling avoids public ingress on the Mac Mini but requires backoff, rate-limit handling, and usage telemetry. Limiting Vision preserves the free tier but trades comprehensive image coverage for predictable cost.

**Evidence:** Flipify's public reference documents `GET /listings/search`, `GET /listings/{listing_id}`, `POST /listings/{listing_id}/enrich`, plus account-credit and usage endpoints; Google Vision bills feature units independently.

**Follow-ups:** Once `FLIPIFY_API_KEY` is available, record the live account allowance and observed per-operation cost before reducing the five-minute interval.

> **[CROSS-LOG]** Product impact logged in `./docs/product_insights.md` — see [PI-ROADMAP] | 2026-08-25: Personal-first hybrid MVP on a Mac Mini.

---

<!-- SESSION: 2026-08-25 00:00 | apartment hunter ingestion review -->
### [CP-CONSTRAINT] | 2026-08-25: Flipify uses credit-metered API polling, not webhooks

**Summary:** Flipify's published v1 API exposes live listing search, basic listing retrieval, and per-listing enrichment; it does not document webhook delivery. A live-search page costs one credit even when it returns no listings, while enrichment costs one credit per listing.

**Files/Modules Affected:** `files/apartment-hunter-handoff.md` and `files/IMPLEMENTATION_CHECKLIST.md` require a future revision before implementation; no product code exists yet.

**Key Trade-off:** Prefer a scheduled, credit-budgeted API poller on the Mac Mini over a public webhook receiver. This protects home-network exposure but makes freshness dependent on a polling cadence that fits the credit allowance.

**Evidence:** Verified in Flipify's public API reference: `GET /listings/search`, `GET /listings/{listing_id}`, and `POST /listings/{listing_id}/enrich` are the documented listing operations and their initial credit charges are stated there.

**Follow-ups:** Choose the number of search profiles and polling interval only after confirming the account's monthly API-credit allowance and acceptable alert latency.

> **[CROSS-LOG]** Product impact logged in `./docs/product_insights.md` — see [PI-ROADMAP] | 2026-08-25: Credit-budgeted Flipify API ingestion.

---

### [CP-INTEGRATION] | 2026-08-25: Use Resend for qualified-listing alerts

**Summary:** The notification integration now uses Resend transactional email instead of Telegram. Alerts will carry only the information needed to triage a listing and use a listing-scoped idempotency key to prevent retries from creating duplicates.

**Files/Modules Affected:** `files/apartment-hunter-handoff.md`, `files/IMPLEMENTATION_CHECKLIST.md`.

**Key Trade-off:** Email provides a durable, low-maintenance notification and a clear API-integration story, but does not guarantee the immediacy of native push alerts.

**Evidence:** Resend's send-email API supports both a dedicated sending permission and an idempotency key. Its free transactional tier fits the expected personal-alert volume.

**Follow-ups:** Configure and verify the sending domain plus a sending-only key during pre-implementation setup; test one end-to-end alert before enabling real notifications.

> **[CROSS-LOG]** Product decision logged in `./docs/product_insights.md` — see [PI-ROADMAP] | 2026-08-25: Resend email as the MVP alert channel.

---

### [CP-INTEGRATION] | 2026-08-25: Split urgent Telegram alerts from Resend digests

**Summary:** Notifications now use two channels with distinct purposes: Telegram sends only immediate alerts for exceptional listings, while Resend sends a scheduled digest for the complete qualified-listing review record. Delivery is tracked per listing, channel, and notification type to prevent duplicate sends.

**Files/Modules Affected:** `files/apartment-hunter-handoff.md`, `files/IMPLEMENTATION_CHECKLIST.md`.

**Key Trade-off:** The second integration improves urgent-awareness without turning every qualified listing into push noise; it adds bot credentials and a delivery-state table.

**Evidence:** Resend supports idempotency keys for send operations and scheduled digest composition is application-controlled. The project owner selected the two-channel design.

**Follow-ups:** Define and validate the strict urgent-notification rule against manually reviewed listings before enabling Telegram for live searches.

> **[CROSS-LOG]** Product decision logged in `./docs/product_insights.md` — see [PI-ROADMAP] | 2026-08-25: Hybrid urgent-alert and digest strategy.

---

### [CP-ARCHITECTURE] | 2026-08-25: Establish API ownership, scrape targets, and on-demand Claude drafting

**Summary:** Go now owns SQLite and exposes the application API; Flask is a presentation client only. Prometheus targets the Compose service names and the inference sidecar exposes a separate HTTP metrics port. Claude drafting is an explicit UI-triggered Anthropic Messages API call, while Telegram urgency is a deterministic rule over fraud and fit scores.

**Files/Modules Affected:** `files/apartment-hunter-handoff.md`, `files/IMPLEMENTATION_CHECKLIST.md`.

**Key Trade-off:** These boundaries prevent multi-process SQLite writes and invalid Prometheus targets, while on-demand drafting reduces unnecessary third-party calls. The deterministic urgent rule is intentionally conservative until manually reviewed outcomes justify adjustment.

**Evidence:** Docker Compose service discovery addresses services by name; the Anthropic Messages API supports a direct HTTPS integration. The current Anthropic Go SDK requires Go 1.24+, so direct HTTP preserves the Go 1.21 project baseline.

**Follow-ups:** Add a request adapter for Anthropic, a single SQLite repository layer in Go, and boundary tests for Telegram urgency before enabling live credentials.

> **[CROSS-LOG]** Product impact logged in `./docs/product_insights.md` — see [PI-ROADMAP] | 2026-08-25: Claude as an intentional drafting action.

---

### [CP-SCOPE] | 2026-08-25: Defer AI inquiry drafting from the MVP

**Summary:** The user moved Claude/Anthropic inquiry drafting out of the MVP. The architecture and checklist now contain no Anthropic credential, HTTP endpoint, package, cost metric, dashboard series, test, or release criterion. A single deferred-item note preserves the option for later reconsideration.

**Files/Modules Affected:** `files/apartment-hunter-handoff.md`, `files/IMPLEMENTATION_CHECKLIST.md`.

**Key Trade-off:** Removing the drafting integration narrows the initial system to discovery, research-backed risk scoring, review, and notifications. It reduces credentials, external failure modes, cost telemetry, and UI scope, at the expense of a future convenience feature.

**Evidence:** Project owner direction, 2026-08-25.

**Follow-ups:** Do not create Anthropic integration code or configure a credential. Revisit only after the core workflow has been exercised reliably.

> **[CROSS-LOG]** Product scope decision logged in `./docs/product_insights.md` — see [PI-ROADMAP] | 2026-08-25: Defer AI inquiry drafting from the MVP.

---

<!-- SESSION: 2026-08-25 | full Apartment Hunter MVP implementation -->
### [CP-MILESTONE] | 2026-08-25: Implement the Apartment Hunter MVP as a deployable single-host stack

**The Context & Problem:**
- The repository contained only a handoff and checklist. The MVP needed a runnable implementation that keeps listing ingestion, SQLite ownership, risk evaluation, notifications, observability, and review UI within the agreed personal-use scope.

**Design Decisions & Trade-offs:**
- **Choice:** Go is the sole SQLite owner and exposes the application API; Flask is presentation-only and the Python classifier is a separate JSON-over-gRPC sidecar governed by the supplied protobuf contract.
- **Alternatives Considered:** Direct SQLite access from Flask/Python and public webhook delivery were rejected by the existing plan because they create writer contention and home-network exposure.
- **Why:** A single writer plus an API boundary makes idempotency and deployment behavior explicit while retaining the portability of SQLite on a Mac Mini.

**The Pivot/Revision:**
- The implementation retains the plan's five-minute credit-budgeted polling default, one primary image, independent Vision feature caps, strict Telegram gate, and complete Resend digest; it does not add the deferred drafting or optional market-comps features.

**Implementation Notes:**
- **Files/Modules Affected:** Go service under `cmd/` and `internal/`; Python sidecar under `inference-sidecar/`; Flask UI under `web/`; Compose, Prometheus, Grafana, documentation, CI, and configuration at the repository root.
- **Core Pattern Introduced:** Persist a basic opaque Flipify ID before enrichment, skip duplicates, evaluate three detectors concurrently, persist explainable results, and reserve durable Vision units in the cost ledger before API calls. Notification delivery state and provider idempotency keys prevent ordinary retries from duplicating alerts.

**Verification & Evidence:**
- `go test ./...` and `go vet ./...` pass in Go 1.24 Docker tooling; provider-boundary tests cover Flipify search/enrichment and Resend digest safety/deduplication. The Python sidecar image builds and its unit test passes. Final Go, sidecar, and Flask images build successfully; local API/UI smoke checks rendered the dashboard and verified `/healthz`, Prometheus metrics, and immediate cost-ledger metric refresh.

**Documentation & References Utilized:**
- `files/apartment-hunter-handoff.md` and `files/IMPLEMENTATION_CHECKLIST.md` — implementation contract and MVP scope.

**Code Snapshot/Diff Concept:**
- `Flipify search → INSERT OR IGNORE(source_id) → enrich fresh row → parallel signals → SaveEvaluation → Telegram if strict urgent / Resend in scheduled digest`.

**Cross-Log:**
- Product implementation impact in `product_insights.md` — see [PI-ROADMAP] | 2026-08-25: Deliver the review-first MVP without enabling live providers.

**Open Questions / Follow-ups:**
- Configure real Flipify, Google Vision, Resend, and Telegram credentials; record the live Flipify allowance before lowering the polling interval; mount and validate the selected ONNX model against a held-out labeled dataset before relying on its signal.

---
