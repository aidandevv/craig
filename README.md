# Apartment Hunter

Finding a rental in San Francisco means sorting through listings that may look promising before there is enough evidence to trust them. Apartment Hunter collects Craigslist rentals through Flipify, scores each listing against the renter's preferences, and checks for fraud risk signals. Low-risk listings enter a dashboard and email digest; the strongest preference matches can also trigger a Telegram alert.

The system runs on one machine, starting with a Mac Mini. Docker Compose keeps the same setup reproducible for a later VPS move.

## From search result to review queue

1. **Find and score listings.** The poller searches through the approved Flipify API, keeps each provider listing ID, and enriches a new listing once. Price, location, and bedroom preferences determine its `fit_score`; an AI model does not make that decision.
2. **Check the available evidence.** Google Vision checks one primary image for reverse-image matches and text, while an ONNX model supplies a separate AI-image signal. These checks run in parallel. If a provider is unavailable, the listing is still saved and the signal records the gap.
3. **Review low-risk candidates.** Listings below the qualification risk threshold appear in the dashboard. Telegram is reserved for listings with `fraud_score <= 0.20`, `fit_score >= 0.80`, and no hard risk flag. Every qualified listing enters the next Resend digest, so the email remains the complete review record.

SQLite stores listings, scores, notification state, provider usage, and costs. The Go service is the only process that opens the database. Vision Web Detection and OCR have separate monthly budgets, each capped below 1,000 units.

A risk score is a reason to inspect a listing, not a verdict about the person who posted it. Provider gaps can leave a check incomplete, so a low score is not proof that a listing is safe.

AI inquiry drafting, direct marketplace scraping, and BatchData comps are outside this MVP.

## Run it locally

1. Copy `.env.example` to `.env` and add the credentials you plan to use. Keep `.env` out of Git.
2. Before enabling the poller, check the live Flipify allowance through `/account/credits` and `/account/usage`. Keep `FLIPIFY_POLL_INTERVAL=5m` until the observed allowance supports more frequent polling.
3. Start the stack:

   ```sh
   docker compose up --build
   ```

4. Open the [review dashboard](http://localhost:5000), [service health endpoint](http://localhost:8080/healthz), [Prometheus](http://localhost:9091), and [Grafana](http://localhost:3000).

Without `FLIPIFY_API_KEY`, the poller stays disabled while the API, dashboard, metrics, and sidecar start. Without a mounted ONNX model, the sidecar reports that the model is unavailable and the rest of the pipeline continues.

## Verify the build

The Docker build runs the Go tests and compiles the service. The sidecar test command checks the Python inference service.

```sh
docker build --target build .
docker compose run --rm inference-sidecar python -m pytest
```

## Configure providers

- **Flipify:** set `FLIPIFY_API_KEY`. The client uses `/listings/search`, `/listings/{id}/enrich`, `/account/credits`, and `/account/usage`.
- **Google Vision:** set a restricted `GOOGLE_VISION_API_KEY`, or mount a service account and set `GOOGLE_APPLICATION_CREDENTIALS`. Keep both Vision feature caps below 1,000 units per month.
- **ONNX:** mount a compatible model as `models/sentry-image.onnx`. The adapter expects a standard 224px NCHW classifier; verify the model's input and AI-class index before using its signal.
- **Resend and Telegram:** delivery stays disabled until the required credentials are set. A delivery failure does not block SQLite persistence.

For implementation and operation details, see [Architecture](ARCHITECTURE.md), [deployment](DEPLOYMENT.md), [cost guardrails](COSTS.md), [API reference](API.md), and [design decisions](DESIGN_DECISIONS.md).
