# craig-extension

A local-first scam detector for online marketplace listings.

You browse Craigslist normally. A Chrome extension reads the listing on screen
and hands it to a daemon running on your own machine, which scores it against a
rule set you control and explains every point of that score. Nothing is sent
anywhere except the image-analysis calls you configure with your own API key.

> Status: Phase 1 complete. The offline rule engine and CLI are ready; the
> daemon and Chrome extension remain planned in [docs/design.md](docs/design.md).

## Why it exists

This is the counterpart to [Apartment Hunter](https://github.com/aidandevv/craig),
which pays an API to *find* listings and filter them down. That is a push model
with a per-listing cost and a fixed idea of what you are shopping for. This
inverts it: you find the listing, the tool judges it, and the expensive parts
only run when you ask.

## Design principles

- **Explainability over a number.** A score with no reasons is not actionable.
  Every rule that fires reports what it matched.
- **A check that could not run is not a check that passed.** Missing API keys and
  exhausted quotas are normal states, and are reported as their own category so a
  listing that could not be verified never looks clean.
- **Signals are indicators, never accusations.** Nothing here proves fraud.
- **Bring your own key.** Google Vision is the headline signal and uses your
  credentials. With no key configured, the pattern and contact rules still run.
- **Green flags cannot move the score.** Reassuring phrases are shown to you but
  contribute nothing, because anything that lowers a score is something a scammer
  will write on purpose.

## What is here now

| Package | Purpose |
|---|---|
| `internal/domain` | `Listing` and `SignalResult`, the two types every layer shares |
| `internal/detect` | `Detector` interface and concurrent fan-out across signals |
| `internal/cache` | SQLite: image-signal results and the durable monthly API budget |
| `internal/signals` | Google Vision reverse-image search and OCR watermark detection |
| `internal/rules` | Embedded/user-defined YAML rules compiled into detectors |
| `internal/risk` | Explainable score, band, coverage, and five result buckets |
| `cmd/craig-extension` | Offline `analyze` command |

No listing is ever persisted. The cache stores image-signal results keyed by a
hash of the image URL, plus a ledger of API units spent this month.

## Try it

```sh
make build
./bin/craig-extension analyze --data '{
  "title": "Beautiful 2BR - MUST GO TODAY",
  "description": "I am out of the country. Send the deposit by western union to hold the unit.",
  "contact": {"relay_only": true}
}'
```

Use `--json` for machine-readable output. Pass a file through `--file` (or
`--file -` for stdin), and use `--rules rules.yaml` to load a custom ruleset.
The default rules work entirely offline; image rules are explicitly reported as
`no_api_key` until Vision is configured in the daemon phase.

## Development

Go is not assumed on the host; everything runs in a container.

```sh
make test     # vet + tests + build, via the Dockerfile build stage
make build    # extract the CLI to ./bin/craig-extension
make format   # gofmt
make tidy     # go mod tidy
```

## Documents

- [docs/design.md](docs/design.md) — full design spec, decisions, and build order
