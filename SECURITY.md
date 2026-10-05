# Extension security boundaries

The Vision key, rules, evidence cache, and usage ledger remain in
`chrome.storage.local`, restricted to trusted extension contexts. Content
scripts receive only the auto-run setting and assessment results. Vision
requests use `X-Goog-Api-Key`; credentials are excluded from URLs and traces.
Only HTTPS image URLs on `images.craigslist.org` are submitted to Vision.

Rule writes require the packaged options page. Listing requests require a
main-frame Craigslist content script and a listing URL matching its sender.
Fresh photo checks require a short-lived, single-use grant from a toolbar
click. The result panel reruns local checks with cached photo evidence;
use the toolbar to refresh photos. With auto-run disabled, ordinary content
requests cannot make provider calls on cache misses. Requests are throttled
and bounded.

The browser bridge accepts at most 1 MiB of JSON, 256 rules, 64 entries per
pattern/check list, 1,024 entries in total, and 256,000 bytes of rule text. Listings
allow 24 images, 128 captions, and 400,000 bytes of aggregate text. Analysis calls
have a 60-second deadline; individual storage waits time out after 5 seconds.
The worker and bridge each allow at most four concurrent analyses/calls.

Release builds use read-only repository permissions, disabled credential
persistence and npm lifecycle scripts, and action references pinned to commit
SHAs. Tags must refer to commits reachable from `main`. A separate job has
write access only to create the draft release from the build artifact.
Repository administrators should protect `main` and `web-v*` tags and restrict
who can create release tags; workflow files cannot enforce those settings.

## Reporting a vulnerability

Please report security issues privately, not in a public issue. Use
GitHub's **Report a vulnerability** button on this repository's Security tab,
or email dev@aidandevaney.com with the steps to reproduce and the affected
version. You should receive a reply within a few days, and fixes are released
as a new Chrome Web Store version.
