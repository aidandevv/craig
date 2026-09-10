import assert from "node:assert/strict";
import test from "node:test";
import { JSDOM } from "jsdom";

import { renderAssessment, renderError } from "../content/badge";
import type { Assessment } from "../shared/types";

const assessment: Assessment = {
	risk_score: 0.73,
	risk_band: "elevated",
	hard_flagged: false,
	coverage: { ran: 4, enabled: 7 },
	high_risk: [{ rule: "gift_card", label: "Gift-card payment requested", detail: "matched \"gift cards\"", weight: .45 }],
	potentially_risky: [{ rule: "absentee", label: "<img src=x onerror=alert(1)>", detail: "owner is away", weight: .28 }],
	positive_signals: [{ rule: "direct_phone", label: "Direct phone" }],
	passed_checks: ["contact_evasion"],
	not_evaluated: [{ rule: "reverse_image", reason: "no_api_key" }],
	analysis_time_ms: 12
};

test("badge keeps coverage and meaningful risk groups visible without injecting daemon text as HTML", () => {
	const dom = new JSDOM("<!doctype html><html><body></body></html>");
	renderAssessment(dom.window.document, assessment, false, () => undefined);
	const shadow = dom.window.document.querySelector("#craig-extension-badge")?.shadowRoot;
	assert.ok(shadow);
	assert.match(shadow.textContent || "", /4 of 7 checks ran/);
	assert.match(shadow.textContent || "", /High risk \(1\)/);
	assert.match(shadow.textContent || "", /Not evaluated \(1\)/);
	assert.equal(shadow.querySelector("img"), null);
	assert.equal(shadow.querySelectorAll("details.check-case").length, 6);
	assert.equal(shadow.querySelectorAll("details[open]").length, 2);
});

test("badge renders reverse-image evidence as a safe image pair and source URL", () => {
	const dom = new JSDOM("<!doctype html><html><body></body></html>");
	const withImageEvidence: Assessment = {
		...assessment,
		high_risk: [{
			rule: "reverse_image_real_estate",
			label: "Listing photo also appears on a real-estate site",
			detail: "image 2 — matched \"zillow\"",
			image_matches: [{
				listing_image_url: "https://images.example/listing.jpg",
				source_page_url: "https://www.zillow.example/home/123",
				source_image_url: "https://images.example/source.jpg"
			}]
		}]
	};
	renderAssessment(dom.window.document, withImageEvidence, false, () => undefined);
	const shadow = dom.window.document.querySelector("#craig-extension-badge")?.shadowRoot!;
	assert.equal(shadow.querySelectorAll(".image-pair img").length, 2);
	const source = shadow.querySelector<HTMLAnchorElement>(".match-source");
	assert.equal(source?.href, "https://www.zillow.example/home/123");
	assert.equal(source?.target, "_blank");
	assert.equal(source?.rel, "noopener noreferrer");
});

test("badge renders a successful legacy assessment with null empty groups", () => {
	const dom = new JSDOM("<!doctype html><html><body></body></html>");
	const legacyAssessment = {
		...assessment,
		high_risk: null,
		potentially_risky: null,
		positive_signals: null,
		passed_checks: null,
		not_evaluated: null
	} as unknown as Assessment;
	assert.doesNotThrow(() => renderAssessment(dom.window.document, legacyAssessment, false, () => undefined));
	const shadow = dom.window.document.querySelector("#craig-extension-badge")?.shadowRoot;
	assert.ok(shadow);
	assert.match(shadow.textContent || "", /73\/100 · Use High Caution/);
});

test("badge renders a collapsed trace with only safe event text", () => {
	const dom = new JSDOM("<!doctype html><html><body></body></html>");
	renderAssessment(dom.window.document, assessment, false, () => undefined, [
		{ timestamp: "2026-09-09T12:34:56.789Z", step: "vision", message: "reverse_image image 2/4: calling Google Vision WEB_DETECTION" }
	]);
	const shadow = dom.window.document.querySelector("#craig-extension-badge")?.shadowRoot!;
	const trace = [...shadow.querySelectorAll("details")].find((element) => element.textContent?.includes("Verified execution log"));
	assert.ok(trace);
	assert.equal(trace.open, false);
	assert.match(trace.textContent || "", /image 2\/4/);
	assert.doesNotMatch(trace.textContent || "", /Bearer|API key/);
});

test("badge calls the options callback when configuration is needed", () => {
	const dom = new JSDOM("<!doctype html><html><body></body></html>");
	let opened = 0;
	renderError(dom.window.document, "Could not reach the local daemon at http://127.0.0.1:8765.", () => { opened++; });
	const shadow = dom.window.document.querySelector("#craig-extension-badge")?.shadowRoot!;
	(shadow.querySelector("button") as HTMLButtonElement).click();
	assert.equal(opened, 1);
	assert.match(shadow.textContent || "", /Local daemon unavailable/);
});
