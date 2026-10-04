import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { JSDOM } from "jsdom";

const site = "https://craig.aidandevaney.com/";

test("manifest points the Chrome Web Store and extension page at the Craig website", async () => {
	const manifest = JSON.parse(await readFile("manifest.json", "utf8"));
	assert.equal(manifest.homepage_url, site);
});

test("options page links to the hosted privacy policy in a new, isolated tab", async () => {
	const dom = new JSDOM(await readFile("options/index.html", "utf8"));
	const link = dom.window.document.querySelector<HTMLAnchorElement>(`a[href="${site}privacy"]`);
	assert.ok(link, "privacy link is present");
	assert.match(link.textContent || "", /privacy policy/i);
	assert.equal(link.target, "_blank");
	assert.equal(link.rel, "noopener noreferrer");
});
