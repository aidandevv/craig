# Product Insight Journal
> Chronological log of user pain points, UX friction, product hypotheses, positioning insights, roadmap trade-offs, experiments, and product decisions.

---

<!-- SESSION: 2026-08-25 00:00 | apartment hunter scope review -->
### [PI-ROADMAP] | 2026-08-25: Personal-first hybrid MVP on a Mac Mini

**Observation:**
- The owner wants Apartment Hunter to be both a personal apartment-finding tool and a portfolio showcase, with a Mac Mini as the preferred deployment target and throughput below 1,000 Vision units per month.

**Why It Matters:**
- The MVP should prioritize trustworthy, fast daily review and a demonstrable architecture over a production-scale, always-public multi-service deployment. Keeping API usage below the first monthly Vision tier supports cost control.

**Evidence:**
- **Source:** Project owner direction during implementation-handoff review.
- **Strength:** Strong

**Hypothesis / Next Step:**
- Build a Mac Mini-first, human-in-the-loop review workflow; retain portable Docker deployment and documented observability as portfolio evidence, but defer public hosting and nonessential integrations.

---

### [PI-ROADMAP] | 2026-08-25: Use Flipify for listing ingestion

**Observation:**
- The owner selected Flipify as the Craigslist listing-ingestion provider instead of building a custom crawler.

**Why It Matters:**
- This keeps the project within a maintainable, policy-aligned scope and focuses the portfolio work on evaluation, review, and operational reliability rather than marketplace scraping.

**Evidence:**
- **Source:** Project owner direction during implementation-handoff review.
- **Strength:** Strong

**Hypothesis / Next Step:**
- Verify Flipify's current Craigslist API contract and choose its supported delivery mode (API polling or authenticated webhook) before implementing the ingestion adapter.

---

### [PI-ROADMAP] | 2026-08-25: Credit-budgeted Flipify API ingestion

**Observation:**
- Flipify's documented v1 integration is API polling rather than webhook delivery. A live-search page costs one credit even when it returns no listings; a listing enrichment also costs one credit.

**Why It Matters:**
- A minute-level poll would consume about 43,200 search credits/month for one profile before any enrichment. The personal tool therefore needs a deliberate freshness-versus-cost budget rather than an assumed real-time webhook.

**Evidence:**
- **Source:** Flipify public API reference, reviewed 2026-08-25.
- **Strength:** Strong

**Hypothesis / Next Step:**
- Confirm the account's included API-credit allowance, then allocate it between a small number of search profiles and enrichment only for candidates worth reviewing.

> **[CROSS-LOG]** Engineering constraint logged in `./docs/dev_journal.md` — see [CP-CONSTRAINT] | 2026-08-25: Flipify uses credit-metered API polling, not webhooks.

---

### [PI-ROADMAP] | 2026-08-25: Start with five-minute Flipify polling and capped Vision review

**Observation:**
- The implementation plan now starts with a five-minute Flipify API polling interval, enrichment for newly discovered listings, and one-image Vision evaluation for candidate listings. The interval may reduce to two minutes after the account allowance is verified.

**Why It Matters:**
- This makes the Mac Mini deployment private and predictable while preserving a clear path to faster notification. The separate Vision cap prioritizes a useful daily review queue over exhaustive image processing.

**Evidence:**
- **Source:** Project owner direction and Flipify public API reference, reviewed 2026-08-25.
- **Strength:** Strong

**Hypothesis / Next Step:**
- Record actual Flipify API usage and Vision feature units during the first month, then decide whether faster polling or broader image analysis improves the apartment-search outcome enough to justify it.

> **[CROSS-LOG]** Engineering implementation logged in `./docs/dev_journal.md` — see [CP-PIVOT] | 2026-08-25: Implement Flipify API polling with a Vision budget cap.

---

### [PI-ROADMAP] | 2026-08-25: Resend email as the MVP alert channel

**Observation:**
- The owner replaced Telegram with Resend transactional email for new qualified-listing alerts.

**Why It Matters:**
- Email creates a searchable review trail while keeping the personal tool and portfolio integration simple. The alert should prompt rapid review, not carry sensitive listing data or replace the dashboard.

**Evidence:**
- **Source:** Project owner direction and Resend product documentation, reviewed 2026-08-25.
- **Strength:** Strong

**Hypothesis / Next Step:**
- Send a concise, deduplicated email for each newly qualified listing and evaluate whether inbox delivery is fast enough before adding a push-notification channel.

> **[CROSS-LOG]** Engineering integration logged in `./docs/dev_journal.md` — see [CP-INTEGRATION] | 2026-08-25: Use Resend for qualified-listing alerts.

---

### [PI-ROADMAP] | 2026-08-25: Hybrid urgent-alert and digest strategy

**Observation:**
- The owner selected Telegram for exceptional listings requiring immediate review and Resend for a scheduled digest covering every qualified listing.

**Why It Matters:**
- This protects attention: only the most valuable, low-risk leads interrupt the user, while email preserves a complete searchable queue for normal review.

**Evidence:**
- **Source:** Project owner direction, reviewed 2026-08-25.
- **Strength:** Strong

**Hypothesis / Next Step:**
- Start with a strict urgent rule and inspect the first month of urgent-alert precision. Relax the rule only if significant legitimate opportunities are being missed.

> **[CROSS-LOG]** Engineering integration logged in `./docs/dev_journal.md` — see [CP-INTEGRATION] | 2026-08-25: Split urgent Telegram alerts from Resend digests.

---

### [PI-ROADMAP] | 2026-08-25: Claude as an intentional drafting action

**Observation:**
- Claude is reserved for drafting a landlord inquiry after the user has reviewed a listing and explicitly requests it; it is not part of automated intake, scoring, or alerting.

**Why It Matters:**
- This keeps the main review loop explainable and avoids producing low-value drafts for listings the user has not selected. It also gives the user final control before personal profile details are sent to the model.

**Evidence:**
- **Source:** Project design decision, reviewed 2026-08-25.
- **Strength:** Strong

**Hypothesis / Next Step:**
- Measure whether user-requested drafts reduce reply time or editing effort before considering any more automated drafting behavior.

> **[CROSS-LOG]** Engineering implementation logged in `./docs/dev_journal.md` — see [CP-ARCHITECTURE] | 2026-08-25: Establish API ownership, scrape targets, and on-demand Claude drafting.

---

### [PI-ROADMAP] | 2026-08-25: Defer AI inquiry drafting from the MVP

**Observation:**
- The owner explicitly moved AI-generated landlord-inquiry drafts out of scope for now.

**Why It Matters:**
- The personal workflow's immediate value is finding, evaluating, and reliably surfacing promising apartments. Deferring drafting keeps the first version focused and avoids adding a third-party model dependency before the review loop proves useful.

**Evidence:**
- **Source:** Project owner direction, reviewed 2026-08-25.
- **Strength:** Strong

**Hypothesis / Next Step:**
- Reconsider an optional drafting feature only after the user has a dependable discovery, review, and notification workflow and can identify a real need it would solve.

> **[CROSS-LOG]** Engineering scope decision logged in `./docs/dev_journal.md` — see [CP-SCOPE] | 2026-08-25: Defer AI inquiry drafting from the MVP.

---

### [PI-ROADMAP] | 2026-08-25: Deliver the review-first MVP without enabling live providers

**Observation:**
- The implemented local stack now presents qualified listings, explainable risk details, review-status actions, urgent-alert gating, and a complete digest path. It starts safely with all external providers disabled until the owner supplies credentials and verifies account limits.

**Why It Matters:**
- The user can validate the intended daily review experience and portfolio architecture without accidentally spending credits, sending notifications, or exposing a home machine. The controlled activation path reduces trust and feasibility risk before live apartment-search data is introduced.

**Evidence:**
- **Source:** Local API/UI smoke test and built Docker images, 2026-08-25.
- **Strength:** Medium

**Hypothesis / Next Step:**
- Configure one Flipify search and run a small monitored live trial. Measure whether the qualified queue is reviewable in under ten minutes a day and whether the strict urgent rule has acceptable precision before changing cadence or thresholds.

> **[CROSS-LOG]** Engineering implementation logged in `./docs/dev_journal.md` — see [CP-MILESTONE] | 2026-08-25: Implement the Apartment Hunter MVP as a deployable single-host stack.

---
