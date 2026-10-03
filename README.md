# craig-extension

A local-first scam detector for online marketplace listings.

You browse Craigslist normally. The Craig Chrome extension reads the listing on
screen and scores it against a rule set you control, inside your browser, then
explains every point of that score. Listing text never leaves your computer.
The only network calls are optional photo checks, sent to Google Cloud Vision
with your own API key.

> Status: release candidate for the Chrome Web Store. The analysis engine runs
> in the extension as WebAssembly, so there is nothing else to install. Not yet
> published; see [Install](#install) to load it from source.

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
| `internal/engine` | One analysis pipeline shared by the extension and the CLI |
| `internal/browserapi` | JSON boundary between the extension and the WebAssembly engine |
| `internal/signals` | Google Vision reverse-image search and OCR watermark detection |
| `internal/rules` | Embedded/user-defined YAML rules compiled into detectors |
| `internal/marketdata` | Versioned offline HUD Small Area Fair Market Rent benchmarks |
| `internal/risk` | Explainable score, band, coverage, and five result buckets |
| `internal/marketplace` | Listing payload validation and normalization |
| `internal/cache`, `internal/config`, `internal/daemon` | SQLite cache, private config, and optional localhost API for the CLI build |
| `cmd/craig-wasm` | WebAssembly entry point loaded by the extension |
| `cmd/craig-extension` | Offline `analyze` command and optional local daemon |
| `extension` | Manifest V3 Craigslist adapter, in-browser engine host, result badge, and options page |

No listing is ever persisted. Craig stores only Vision results for photos
checked in the last 24 hours, keyed by image URL, plus a count of photo checks
used this month. [PRIVACY.md](PRIVACY.md) lists exactly what is stored and sent.

## Install

Until the Chrome Web Store listing is live, build the extension from source.
You need Docker (Go is not assumed on the host) and Node 22.

```sh
make wasm               # compile the engine to extension/generated/engine.wasm
make extension-install
make extension-build
```

Open `chrome://extensions`, turn on **Developer mode**, choose **Load
unpacked**, and select the repository's `extension` directory. Open any
Craigslist listing and click the Craig toolbar icon to analyze it.

Release packages are built by CI: pushing a `web-v<version>` tag that matches
`extension/manifest.json` creates a draft GitHub release containing the zip to
upload to the Chrome Web Store.

## Using the extension

Craig works without any setup. Text, contact, fee, rent, and HUD checks run
immediately; photo checks are reported as **not evaluated** until you add a
key.

To turn on photo checks, open the extension's **Options** and follow the five
steps under **Photo checks** to create a Google Cloud Vision API key. Restrict
the key to the Cloud Vision API and set a budget alert. The key is stored only
in this browser profile's extension storage and is sent only to Google. The
same page sets how many photos to check per listing (default 4), a monthly
limit per check type, and whether to analyze every listing automatically when
it opens (off by default).

Each uncached photo uses one `WEB_DETECTION` request (shared by the reverse
image and stock-photo rules) and one `TEXT_DETECTION` OCR request (for the MLS
watermark rule). Only photos from `images.craigslist.org` are sent. Results are
cached for 24 hours and re-scored against your current rules on every analysis.
The badge's **Check again** button bypasses that cache while still respecting the
monthly limit. Failed photos are reported as incomplete. Only matching pages
and full/partial image matches are used; visually similar suggestions are
ignored.

The badge always shows coverage, so a listing that could not be fully checked
never looks clean. Every result also has a collapsed **Verified execution
log** showing the extension, engine, detector, cache, and Vision stages for
that request. It omits credentials, listing text, and image URLs.

The Options page also has a rule builder for curated pattern checks. Saved
rules are validated by the engine and kept in extension storage. Custom,
contact, image, and regex rules are preserved as read-only cards when a visual
edit is saved.

## Default checks

### Rental fee checks

The default offline fee checks are deliberately caution-only and reported
separately: any hold/reservation fee, key money, option/commitment/priority/
waitlist fee, application or good-faith deposit, or pre-lease fee is
nonstandard; familiar fees are flagged only when unusually high. The default
application-fee threshold is $50; screening, processing, administrative,
lease-initiation, move-in, amenity, convenience, and pet fees use their own
generous ceilings. “Charge” and “cost” are treated like “fee.” A fee alone can
never create a hard flag. Adjust `application_fee_high_threshold` in the rule
builder or your rules YAML if a different local application threshold fits
your market.

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

The bundled snapshot is [HUD FY 2027 Small Area Fair Market Rents](https://www.huduser.gov/portal/datasets/fmr/fmr2027/FY27_safmrs.xlsx), effective October 1, 2026. A SAFMR is HUD's ZIP-level estimate of the 40th-percentile **gross** rent, including estimated tenant-paid utilities. It is not a median asking rent, an appraisal, a live comparable set, or evidence of fraud. The rule is intentionally conservative: only rents at or below 65% of the benchmark add a review cue, and its explanation says exactly which HUD rate was used.

Refresh the embedded asset whenever HUD publishes a new effective schedule;
the source workbook is not committed:

```sh
python3 \
  scripts/build_hud_safmr_data.py /path/to/FY27_safmrs.xlsx \
  internal/marketdata/fy2027_selected.json
```

## Command line and local daemon

The same engine also builds as a native CLI, useful for testing rules without a
browser.

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
`no_api_key` unless a Vision key is configured.

Recorded offline run with the embedded rules and no Vision key:

```text
Risk 1.00  HIGH  (hard flag)
11 of 16 checks ran in 0ms

HIGH RISK
  • Owner claims to be unreachable in person
  • Payment requested by a method with no recourse

POTENTIALLY RISKY
  • Seller reachable only through the marketplace
  • High-pressure language discouraging due diligence

NOT EVALUATED (5)
  market_rent_below_hud            market_rent_inputs_missing
  mls_watermark                    no_api_key
  rent_price_mismatch              rent_price_inputs_missing
  reverse_image_real_estate        no_api_key
  stock_photos                     no_api_key
```

`craig-extension config init` and `craig-extension daemon` still provide the
authenticated localhost API (`127.0.0.1:8765`) from earlier builds. The
extension does not use it.

## Development

Go is not assumed on the host; Go builds run in a container.

```sh
make test            # Go vet + tests + build, via the Dockerfile build stage
make wasm            # build the WebAssembly engine for the extension
make build           # extract the CLI to ./bin/craig-extension
make format          # gofmt
make tidy            # go mod tidy
make extension-test  # fixture, badge, engine, and storage tests
make extension-build # typecheck and build the unpacked MV3 extension
```
