import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { JSDOM } from "jsdom";

import { extractListing } from "../content/craigslist";

test("extractListing normalizes a saved Craigslist listing fixture", async () => {
  const fixture = await readFile(new URL("./fixtures/craigslist-listing.html", import.meta.url), "utf8");
  const dom = new JSDOM(fixture, { url: "https://sfbay.craigslist.org/apa/d/oakland-sunny-2br/123.html" });
  const listing = extractListing(dom.window.document);

  assert.equal(listing.marketplace, "craigslist");
  assert.equal(listing.listing_url, "https://sfbay.craigslist.org/apa/d/oakland-sunny-2br/123.html");
  assert.equal(listing.title, "Sunny 2BR near the park");
  assert.equal(listing.price, 2400);
	assert.equal(listing.posted_at, "2026-09-09T19:00:00.000Z");
  assert.equal(listing.contact.phone, "510-555-1234");
  assert.equal(listing.contact.relay_only, undefined);
  assert.deepEqual(listing.captions, ["Bright kitchen"]);
  assert.equal(listing.images?.[0], "https://images.craigslist.org/00A0A_example_full.jpg");
  assert.equal(listing.description?.includes("QR Code Link"), false);
  assert.match(listing.description || "", /application fee details.*holding fee/is);
});

test("extractListing marks an otherwise anonymous reply route as relay only", () => {
  const dom = new JSDOM(`
    <title>Small studio - craigslist</title>
    <a class="reply-button">reply</a>
    <section id="postingbody">No calls please.</section>
  `, { url: "https://sfbay.craigslist.org/apa/d/oakland-studio/456.html" });
  const listing = extractListing(dom.window.document);

  assert.equal(listing.contact.relay_only, true);
  assert.equal(listing.contact.phone, undefined);
});
