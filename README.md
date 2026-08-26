# Apartment Hunter

Apartment Hunter is a single-machine review system for SF Craigslist rentals. It polls the approved Flipify API, enriches new listings exactly once, runs parallel fraud-risk signals, and presents only qualified candidates in a small dashboard. Exceptional low-risk, high-fit matches can reach Telegram immediately; every qualified listing appears once in the next Resend digest.

It is designed for a Mac Mini first, with Docker Compose keeping a later VPS move reproducible.

## What it does

- Keeps Flipify's opaque listing ID and deduplicates before enrichment.
- Uses one primary image per candidate for independent Google Vision Web Detection and OCR budgets, each capped below 1,000 monthly units.
- Evaluates reverse-image, OCR, and AI-image signals concurrently; missing providers degrade a signal rather than blocking persistence.
- Stores listings, scores, notifications, feature-unit usage, and costs in SQLite. Only the Go service opens the database.
- Uses deterministic price/location/bedroom preferences for `fit_score`, not an AI judgment.
- Limits Telegram to `fraud_score <= 0.20`, `fit_score >= 0.80`, and no hard risk flag. Resend is the complete review record.

AI inquiry drafting, direct marketplace scraping, and BatchData comps are intentionally out of scope for this MVP.

## Quick start

1. Copy `.env.example` to `.env`, add only the credentials you are ready to use, and do not commit it.
2. Before enabling the poller, verify the live Flipify allowance through `/account/credits` and `/account/usage`. Keep `FLIPIFY_POLL_INTERVAL=5m` until the observed allowance supports more frequent polling.
3. Start the local stack:

   ```sh
   docker compose up --build
   ```

4. Open the dashboard at [http://localhost:5000](http://localhost:5000), service API at [http://localhost:8080/healthz](http://localhost:8080/healthz), Prometheus at [http://localhost:9091](http://localhost:9091), and Grafana at [http://localhost:3000](http://localhost:3000).

Without `FLIPIFY_API_KEY`, the poller remains safely disabled while the API, dashboard, metrics, and sidecar start normally. Without a mounted ONNX model, the sidecar returns an explicit unavailable model result and keeps the rest of the pipeline live.

## Test and format

The repository uses a Docker build because the project requires Go 1.21+ and the local host toolchain is not assumed.

```sh
docker build --target build .
docker build --target build .
docker compose run --rm inference-sidecar python -m pytest
```

## Provider setup

- **Flipify:** configure `FLIPIFY_API_KEY`; the client uses `/listings/search`, `/listings/{id}/enrich`, `/account/credits`, and `/account/usage`.
- **Google Vision:** use a restricted `GOOGLE_VISION_API_KEY` or mount a service account and set `GOOGLE_APPLICATION_CREDENTIALS`. Keep both Vision feature caps under 1,000 units per month.
- **ONNX:** mount a compatible model as `models/sentry-image.onnx`. The model adapter assumes a standard 224px NCHW classifier; validate the selected model's input and AI-class index before going live.
- **Resend/Telegram:** both clients stay disabled until all required credentials are set. They never block SQLite persistence.

See [Architecture](ARCHITECTURE.md), [deployment](DEPLOYMENT.md), [cost guardrails](COSTS.md), [API reference](API.md), and [design decisions](DESIGN_DECISIONS.md).
