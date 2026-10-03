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
	assert.equal(listing.rent_period, "monthly");
	assert.equal(listing.bedrooms, 2);
	assert.equal(listing.zip_code, "94103");
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

test("extractListing does not mistake Craigslist's revealed phone control for relay-only contact", () => {
	const dom = new JSDOM(`
		<title>1 BD $1,795 - craigslist</title>
		<button class="reply-button">reply</button>
		<section id="postingbody">Call now <a class="show-contact" href="#">show contact info</a> x 12
		OR Text 12 to <a class="show-contact" href="#">show contact info</a>.</section>
	`, { url: "https://sfbay.craigslist.org/apa/d/san-francisco/789.html" });
	const listing = extractListing(dom.window.document);

	assert.equal(listing.contact.relay_only, undefined);
	assert.equal(listing.contact.phone, undefined, "Craigslist reveals the number only after a user action");
});

test("extractListing recognizes a studio bedroom count", () => {
	const dom = new JSDOM(`<title>Sunny studio near BART - craigslist</title>`, {url: "https://sfbay.craigslist.org/apa/3.html"});
	assert.equal(extractListing(dom.window.document).bedrooms, 0);
});

test("extractListing recognizes bedroom count phrasing variations", () => {
	const cases: Array<[string, number]> = [
		["1bd", 1],
		["1 bd", 1],
		["1bdrm", 1],
		["1bed", 1],
		["1 bedroom", 1],
		["2-Bedroom", 2],
		["2BR", 2],
		["3 bedrooms", 3],
		["4 BR", 4],
	];
	for (const [phrase, expected] of cases) {
		const dom = new JSDOM(`<title>Charming ${phrase} apartment - craigslist</title>`, {
			url: "https://sfbay.craigslist.org/apa/d/example/2.html",
		});
		assert.equal(extractListing(dom.window.document).bedrooms, expected, phrase);
	}
});

test("extractListing sources bedrooms, rent period and fee details from Craigslist's real nested .attrgroup markup even when the title has no bedroom count", () => {
	const dom = new JSDOM(`
		<title>Charming apartment downtown - craigslist</title>
		<div class="attrgroup"><span class="attr important">2BR / 2Ba</span></div>
		<div class="attrgroup">
			<div class="attr application_fee_explained">
				<span class="labl">application fee details:</span>
				<span class="valu">Refundable:'//$100 admin holding fee and +/'$29.95 application fee..</span>
			</div>
			<div class="attr rent_period">
				<span class="labl">rent period:</span>
				<span class="valu"><a href="#">monthly</a></span>
			</div>
		</div>
		<section id="postingbody">Beautiful apartment near downtown.</section>
	`, { url: "https://sfbay.craigslist.org/apa/d/example/1.html" });
	const listing = extractListing(dom.window.document);

	assert.equal(listing.bedrooms, 2);
	assert.equal(listing.rent_period, "monthly");
	assert.match(listing.description || "", /holding fee/i);
});

test("extractListing sources the ZIP code from the street address when it is absent from the title and body", () => {
	const dom = new JSDOM(`
		<title>Lovely unit - craigslist</title>
		<h2 class="street-address">32 Collins St #101, San Francisco, CA 94118</h2>
		<section id="postingbody">Great place, no calls please.</section>
	`, { url: "https://sfbay.craigslist.org/apa/d/example/3.html" });
	assert.equal(extractListing(dom.window.document).zip_code, "94118");
});

test("extractListing upgrades Craigslist thumbnails and supports a single gallery photo", () => {
  for (const html of ['<div id="thumbs"><img src="https://images.craigslist.org/room_50x50.jpg"></div>', '<div class="gallery"><img src="https://images.craigslist.org/room_600x450.jpg"></div>']) {
    const dom=new JSDOM(`<title>Studio</title>${html}`,{url:"https://sfbay.craigslist.org/apa/1.html"});
    assert.deepEqual(extractListing(dom.window.document).images,["https://images.craigslist.org/room_1200x900.jpg"]);
  }
});

test("extractListing collects photos omitted by partial thumbnail links without duplicate size variants", () => {
  const dom=new JSDOM(`<title>Studio</title>
    <div id="thumbs"><a href="https://images.craigslist.org/a_600x450.jpg"><img src="https://images.craigslist.org/a_50x50.jpg"></a>
    <img src="https://images.craigslist.org/b_50x50.jpg"></div>
    <div class="gallery"><img src="https://images.craigslist.org/c_600x450.jpg"><img src="https://images.craigslist.org/a_1200x900.jpg"></div>`,
    {url:"https://sfbay.craigslist.org/apa/1.html"});
  assert.deepEqual(extractListing(dom.window.document).images,[
    "https://images.craigslist.org/a_1200x900.jpg",
    "https://images.craigslist.org/b_1200x900.jpg",
    "https://images.craigslist.org/c_1200x900.jpg"
  ]);
});

test("extractListing merges cropped thumbnails with their full-size photos", () => {
  // Current Craigslist pages pair each _600x450 link with a cropped _50x50c thumbnail.
  const thumbs = Array.from({ length: 13 }, (_, i) =>
    `<a href="https://images.craigslist.org/p${i}_600x450.jpg"><img src="https://images.craigslist.org/p${i}_50x50c.jpg"></a>`).join("");
  const dom=new JSDOM(`<title>Studio</title><div class="gallery"><img src="https://images.craigslist.org/p0_600x450.jpg"></div><div id="thumbs">${thumbs}</div>`,
    {url:"https://www.craigslist.org/view/d/oakland-studio/abc123"});
  const images = extractListing(dom.window.document).images!;
  assert.equal(images.length, 13);
  assert.ok(images.every((url) => url.endsWith("_1200x900.jpg")));
});

test("extractListing sends at most 24 photos", () => {
  const thumbs = Array.from({ length: 30 }, (_, i) => `<a href="https://images.craigslist.org/p${i}_600x450.jpg"></a>`).join("");
  const dom=new JSDOM(`<title>Studio</title><div id="thumbs">${thumbs}</div>`,{url:"https://www.craigslist.org/view/d/oakland-studio/abc123"});
  const images = extractListing(dom.window.document).images!;
  assert.equal(images.length, 24);
  assert.equal(images[0], "https://images.craigslist.org/p0_1200x900.jpg");
});
