# Architecture

```text
Flipify v1 search → Go poller → SQLite (sole owner) → Go API → Flask dashboard
                          │
                          ├─ parallel: Google Vision Web Detection
                          ├─ parallel: Google Vision OCR
                          └─ parallel: gRPC ONNX sidecar
                          │
                          ├─ Telegram (strict urgent rule)
                          └─ Resend (qualified-listing digest)

Go /metrics ──┐
Sidecar /metrics ─┴─> Prometheus → Grafana
```

The poller first inserts a basic listing with Flipify's opaque ID. A unique-ID conflict ends processing before enrichment, so retries and repeated search pages do not create repeat evaluations or external calls. Fresh IDs are enriched once, evaluated in parallel, persisted, and then considered for notifications.

Flask never opens the SQLite file. All reads and review-status writes go to the Go API. The Python sidecar similarly publishes model-evaluation results through `POST /api/model-evaluations`, leaving Go as the only database owner.

The three fraud signals are deliberately explainable risk indicators. A Zillow/Redfin/MLS reverse match or MLS-like watermark is a hard flag. Scores are summed and bounded at 1.0; the operator can inspect flags rather than treating the result as ground truth.
