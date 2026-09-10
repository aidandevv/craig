# craig-extension

A local-first scam detector for online marketplace listings.

You browse Craigslist normally. A Chrome extension reads the listing on screen
and hands it to a daemon running on your own machine, which scores it against a
rule set you control and explains every point of that score. Nothing is sent
anywhere except the image-analysis calls you configure with your own API key.

> Status: MVP complete. The offline engine, secure local daemon, Vision BYOK
> wiring, Craigslist Chrome extension, and visual rule builder are ready.

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
| `internal/config` | Private local config, generated bearer token, and BYOK settings |
| `internal/marketplace` | Listing payload validation and normalization at the API boundary |
| `internal/daemon` | Authenticated localhost HTTP API with rule reload support |
| `cmd/craig-extension` | Offline `analyze` command |
| `extension` | Manifest V3 Craigslist adapter, daemon client, result badge, and options page |

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
`no_api_key` until Vision is configured for the daemon.

### Recorded local run

Excerpt from an actual offline run with the embedded rules and no Vision key:

```text
Risk 1.00  HIGH  (hard flag)
9 of 12 checks ran in 0ms

HIGH RISK
  • Owner claims to be unreachable in person
  • Payment requested by a method with no recourse

POTENTIALLY RISKY
  • Seller reachable only through the marketplace
  • High-pressure language discouraging due diligence

NOT EVALUATED (3)
  mls_watermark                    no_api_key
  reverse_image_real_estate        no_api_key
  stock_photos                     no_api_key
```

## Run the daemon

Initialize once to create a private config file, editable starter rules, and a
random extension token:

```sh
./bin/craig-extension config init
export GOOGLE_VISION_API_KEY='your-restricted-google-vision-key' # optional
./bin/craig-extension daemon --verbose # optional: mirror safe execution traces to this terminal
```

The daemon binds only to `127.0.0.1:8765`. Its API requires the token printed
by `config token` (except `/healthz`), rejects ordinary web-page origins and
unexpected `Host` headers, and accepts `chrome-extension://` origins for the
planned extension. Edit the generated `rules.yaml`; changes are picked up on
the next analysis request, or call `POST /api/reload-rules` explicitly.

With Vision configured, the daemon processes every valid public listing image.
Each uncached image uses one `WEB_DETECTION` request (shared by the reverse
image and stock-photo rules) and one `TEXT_DETECTION` OCR request (for the MLS
watermark rule); cached image results do not call Vision again. Start the daemon
with `--verbose` to mirror the safe execution trace to the terminal. It never
prints your token, Vision key, listing text, or image URLs.

The default offline fee check is deliberately caution-only: it highlights a
stated holding fee or an application fee above $100, but neither can create a
hard flag. Adjust `application_fee_high_threshold` in your generated
`rules.yaml` if a different local threshold fits your market.

## Chrome extension

With the daemon configured and running, build the unpacked extension and load
the repository's `extension` directory from `chrome://extensions` with
Developer mode enabled:

```sh
make extension-install
make extension-build
```

Open the extension's **Options**, paste the value from `craig-extension config
token`, and keep auto-run off until you want every listing checked on page load.
Otherwise, click the extension toolbar icon on a Craigslist listing to analyze
it manually. The badge always shows coverage; it explicitly says when the local
daemon is unreachable or an image check did not run.

Every result also has a collapsed **Verified execution log**. It shows the
actual extension, daemon, detector, cache, and Vision stages for that request,
including each image’s provider call, cache hit, or skip reason. It deliberately
omits credentials, listing text, and image URLs.

The same Options page has a rule builder for curated pattern checks. It
preserves custom, contact, image, and regex rules as read-only cards when a
visual edit is saved. See [docs/custom-rules.md](docs/custom-rules.md) for the
builder and YAML workflows.

## Development

Go is not assumed on the host; everything runs in a container.

```sh
make test     # vet + tests + build, via the Dockerfile build stage
make build    # extract the CLI to ./bin/craig-extension
make format   # gofmt
make tidy     # go mod tidy
make extension-test  # fixture and badge tests
make extension-build # typecheck and build the unpacked MV3 artifact
```

## Documents

- [docs/design.md](docs/design.md) — full design spec, decisions, and build order
- [docs/custom-rules.md](docs/custom-rules.md) — visual builder and YAML guide
