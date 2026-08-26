# Design decisions

## Go owns SQLite

SQLite fits a low-volume, single-host workload, but multiple independent writers create avoidable lock risks. The Go service is therefore the only process that opens the file; Flask and Python call its HTTP API. The cost is an internal API boundary, accepted for data integrity and a clearer deployment story.

## Scheduled Flipify polling, not a webhook receiver

The documented v1 listing flow is search plus enrichment. Polling avoids exposing a home Mac Mini to public inbound traffic but makes freshness consume credits. The initial five-minute setting is intentional and must not be lowered on assumption.

## One primary image and two independent Vision budgets

Web Detection and OCR bill independently. The database reserves each feature unit before a request and caches results by image URL hash. This limits cost predictably; the trade-off is less image coverage until manual review justifies expanding it.

## Explainable scoring and deterministic fit

Signals are independent, parallel, and inspectable. Fraud risk is not a truth claim. Price, location, and bedroom fit are configured deterministic rules so the urgent-notification policy remains auditable.

## Hybrid notifications

Telegram interrupts only for exceptional listings; Resend preserves the complete qualified queue. Persisted listing/channel/type state prevents successful alerts from being repeated after retries.
