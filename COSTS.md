# Cost guardrails

| Provider | Guardrail | How it is enforced |
| --- | --- | --- |
| Flipify | Start at a five-minute poll; lower only after verified credits/rate limits | Preflight calls allowance/usage endpoints and logs the result. |
| Google Vision Web Detection | Fewer than 1,000 monthly images | SQLite ledger reservation before the call, cache by image URL hash. |
| Google Vision OCR | Fewer than 1,000 monthly images, independently | Separate ledger feature and cap. |
| Resend / Telegram | Personal-alert volume | Delivery state avoids duplicate successful sends. |
| Infrastructure | Mac Mini incremental cost | A manually recorded ledger entry supports later operating-cost reporting. |

`cost_ledger` is the source of truth for provider spend and free-tier units. The Grafana cost dashboard surfaces automatically collected Vision-unit/skip counters plus current-month provider totals, a rolling 30-day total, and cost per processed, qualified, and notified listing. Record Flipify, infrastructure, BatchData, and actual paid Vision spend through `POST /api/costs` once live account pricing is known.
