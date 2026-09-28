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
| `internal/marketdata` | Versioned offline HUD Small Area Fair Market Rent benchmarks |
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
10 of 15 checks ran in 0ms

HIGH RISK
  • Owner claims to be unreachable in person
  • Payment requested by a method with no recourse

POTENTIALLY RISKY
  • Seller reachable only through the marketplace
  • High-pressure language discouraging due diligence

NOT EVALUATED (5)

  market_rent_below_hud              market_rent_inputs_missing
  rent_price_mismatch                rent_price_inputs_missing
  mls_watermark                    no_api_key
  reverse_image_real_estate        no_api_key
  stock_photos                     no_api_key
```

## Run the daemon

Initialize once to create a private config file, editable starter rules, and a
random connection code:

```sh
./bin/craig-extension config init
./bin/craig-extension daemon --verbose # optional: mirror safe execution traces to this terminal
```

The helper binds only to `127.0.0.1:8765`. Its API requires the connection code
printed by `config init` (or `config token`) except for `/healthz`, which exposes
only a non-secret helper ID used for local discovery. It rejects ordinary
web-page origins and unexpected `Host` headers, while accepting
`chrome-extension://` origins for Craig. Edit the generated `rules.yaml`;
changes are picked up on the next analysis request, or call
`POST /api/reload-rules` explicitly.

With Vision configured, the daemon processes every valid Craigslist listing image
from `images.craigslist.org`.
Each uncached image uses one `WEB_DETECTION` request (shared by the reverse
image and stock-photo rules) and one `TEXT_DETECTION` OCR request (for the MLS
watermark rule). Successful provider evidence is cached for 24 hours and re-scored
against current rules on every analysis. Clicking the extension explicitly
rechecks photos against Vision, bypassing that cache while respecting the monthly
budget. Failed photos are reported as incomplete. Only matching pages and
full/partial image matches are used; visually similar suggestions are ignored. Start the daemon
with `--verbose` to mirror the safe execution trace to the terminal. It never
prints your token, Vision key, listing text, or image URLs.

The default offline fee checks are deliberately caution-only and reported
separately: any hold/reservation fee, key money, option/commitment/priority/
waitlist fee, application or good-faith deposit, or pre-lease fee is
nonstandard; familiar fees are flagged only when unusually high. The default
application-fee threshold is $50; screening, processing, administrative,
lease-initiation, move-in, amenity, convenience, and pet fees use their own
generous ceilings. “Charge” and “cost” are treated like “fee.” A fee alone can
never create a hard flag. Adjust `application_fee_high_threshold` in your
generated `rules.yaml` if a different local application threshold fits your
market.

### Advertised-rent consistency check

The default rules also compare the page's structured rent with dollar amounts
in its title. A difference of 10% or more creates a small, caution-only prompt
to confirm the current rent before applying or paying a fee. It is designed to
surface stale or bait-and-switch pricing, not to allege fraud, and is not
evaluated when either input is absent.

### HUD market-rent caution check

The default rules also include one offline, caution-only market-rent check
(`weight: 0.15`). It compares a listing only when the page supplies a monthly
USD price, a studio-to-four-bedroom count, and a five-digit ZIP included in the
bundled HUD data. It covers 1,395 ZIP codes across the SF Bay Area, Los Angeles,
and the New York metro area (including Jersey City and Newark). Missing fields
or an unsupported ZIP are shown as **not evaluated**; the engine never falls
back to a citywide rate or geocodes an address.

The bundled snapshot is [HUD FY 2027 Small Area Fair Market Rents](https://www.huduser.gov/portal/datasets/fmr/fmr2027/FY27_safmrs.xlsx), effective October 1, 2026. A SAFMR is HUD's ZIP-level estimate of the 40th-percentile **gross** rent, including estimated tenant-paid utilities. It is not a median asking rent, an appraisal, a live comparable set, or evidence of fraud. The rule is intentionally conservative: only rents at or below 55% of the benchmark add a review cue, and its explanation says exactly which HUD rate was used.

Refresh the embedded asset whenever HUD publishes a new effective schedule;
the source workbook is not committed:

```sh
python3 \
  scripts/build_hud_safmr_data.py /path/to/FY27_safmrs.xlsx \
  internal/marketdata/fy2027_selected.json
```

## Chrome extension

With the daemon configured and running, build the unpacked extension and load
the repository's `extension` directory from `chrome://extensions` with
Developer mode enabled:

```sh
make extension-install
make extension-build
```

Open the extension's **Options** and choose **Find Craig helper**. Craig detects
the local helper and shows its helper ID automatically, so there is no normal
URL setup step. Paste the one-time connection code printed during setup, then
optionally paste a restricted Google Cloud Vision API key under **Photo checks**.
That key is sent directly to the authenticated local helper, saved only in its
private configuration, and is never stored by the browser extension. Keep
auto-run off until you want every listing checked on page load. Otherwise, click
the extension toolbar icon on a Craigslist listing to analyze it manually. The
badge always shows coverage; it explicitly says when the local helper is
unreachable or an image check did not run.

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
make dev             # build the extension and helper, then run the helper verbosely
```

## Documents

- [docs/design.md](docs/design.md) — full design spec, decisions, and build order
- [docs/custom-rules.md](docs/custom-rules.md) — visual builder and YAML guide
