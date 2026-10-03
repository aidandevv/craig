import type { Assessment, Finding, NotEvaluated, TraceEvent } from "../shared/types";

const hostID = "craig-extension-badge";

// Rotates under the loading video so a long-running analysis still reads as progress
// rather than a stall. Held on the module because panelFor() rebuilds the panel's DOM on
// every render, which would otherwise orphan a running interval.
const LOADING_MESSAGES: readonly string[] = [
	"Scanning for common scam patterns…",
	"Giving this listing the side-eye…",
	"Checking contact details and payment requests…",
	"Sniffing out landlords who may not exist…",
	"Comparing photos against known stock and real-estate sources…",
	"Making sure this isn't a ghost ship of a listing…",
	"Cross-checking the price against local rent data…",
	"Reviewing the listing for high-pressure language…"
];
const LOADING_ROTATION_MS = 2200;
let loadingRotationTimer: ReturnType<typeof setInterval> | undefined;

export function renderLoading(document: Document): void {
	const panel = panelFor(document);
	renderHeader(panel, "Checking this listing", "Running local scam checks…");

	const visual = document.createElement("div");
	visual.className = "loading-visual";
	const video = document.createElement("video");
	video.className = "loading-video";
	video.autoplay = true;
	video.loop = true;
	video.muted = true;
	video.playsInline = true;
	video.setAttribute("aria-hidden", "true");
	const source = videoURL();
	if (source) video.src = source;
	visual.append(video);

	const status = el(document, "p", LOADING_MESSAGES[0]);
	status.className = "loading-status";
	visual.append(status);
	panel.append(visual);

	let index = 0;
	loadingRotationTimer = setInterval(() => {
		index = (index + 1) % LOADING_MESSAGES.length;
		status.textContent = LOADING_MESSAGES[index];
	}, LOADING_ROTATION_MS);
}

export function renderError(document: Document, message: string, onOpenOptions: () => void, trace: TraceEvent[] = [], onRetry?: () => void): void {
	const panel = panelFor(document);
	renderHeader(panel, "Analysis unavailable", "The listing could not be analyzed.");
	panel.append(el(document, "p", message));
	appendTrace(document, panel, trace);
	panel.append(actionRow(document, onOpenOptions, onRetry, "Try again"));
}

/** Craig's concern states, least to most concerning, plus one non-band state for thin evidence. */
type ConcernState = "low" | "caution" | "elevated" | "high" | "hard-flag" | "incomplete";

/** Below this fraction of enabled checks actually running, the read isn't confident enough to stand behind. */
const INCOMPLETE_COVERAGE_RATIO = 0.7;

/** A hard signal always wins — it's the strongest thing Craig found, regardless of the band or how much coverage backs it. Next, thin coverage overrides the band, since a tidy score built on a fraction of the checks isn't one to stand behind. Otherwise, the risk band speaks for itself. */
function concernStateFor(assessment: Assessment): ConcernState {
	if (assessment.hard_flagged) return "hard-flag";
	const { ran, enabled } = assessment.coverage;
	if (enabled > 0 && ran / enabled < INCOMPLETE_COVERAGE_RATIO) return "incomplete";
	switch (assessment.risk_band) {
		case "low": return "low";
		case "caution": return "caution";
		case "elevated": return "elevated";
		case "high": return "high";
		default: return "caution";
	}
}

const CONCERN_COPY: Record<ConcernState, { heading: string; subheading: string }> = {
	low: { heading: "Low concern", subheading: "No scam indicators were found." },
	caution: { heading: "Look closer", subheading: "A few minor indicators showed up — worth a look." },
	elevated: { heading: "Verify first", subheading: "Several indicators point to risk here." },
	high: { heading: "High concern", subheading: "Scam indicators, not proof — proceed carefully." },
	"hard-flag": { heading: "Scam likely", subheading: "Hard signal found." },
	incomplete: { heading: "Partial check", subheading: "Too few checks ran to give a confident read." }
};

export function renderAssessment(document: Document, assessment: Assessment, cached: boolean, onOpenOptions: () => void, trace: TraceEvent[] = [], onRecheck?: () => void): void {
	const panel = panelFor(document);
	const score = Math.round(Math.max(0, Math.min(1, assessment.risk_score)) * 100);
	// The engine emits empty result groups as [], but Go can encode absent
	// slices as null. A successful analysis should never fail while rendering
	// its empty sections.
	const highRisk = assessment.high_risk ?? [];
	const potentiallyRisky = assessment.potentially_risky ?? [];
	const positiveSignals = assessment.positive_signals ?? [];
	const passedChecks = assessment.passed_checks ?? [];
	const notEvaluated = assessment.not_evaluated ?? [];
	const concern = concernStateFor(assessment);
	const { heading, subheading } = CONCERN_COPY[concern];
	renderHeader(panel, heading, subheading, concernSpriteURL(concern));

	const coverage = el(document, "p", `${assessment.coverage.ran} of ${assessment.coverage.enabled} checks ran${cached ? " · cached for this browser session" : ""}`);
	coverage.className = "coverage";
	panel.append(coverage);
    for (const images of assessment.image_coverage ?? []) {
        const at = new Date(images.checked_at);
        const when = Number.isNaN(at.valueOf()) || at.getFullYear() < 2000 ? "" : ` · oldest check ${at.toLocaleString()}`;
        panel.append(el(document, "p", `Cloud Vision ${images.signal}: ${images.checked} of ${images.total} photos checked${when}`));
    }

	panel.append(scoreGauge(document, score, assessment.risk_band));
	appendFindings(document, panel, "High risk", highRisk, "danger", "Flagged", true);
	appendFindings(document, panel, "Potentially risky", potentiallyRisky, "caution", "Caution", true);
	appendFindings(document, panel, "Positive signals", positiveSignals, "positive", "Passed", false);
	appendNames(document, panel, "Passed checks", passedChecks, false);
	appendNotEvaluated(document, panel, notEvaluated);
	appendTrace(document, panel, trace);

	panel.append(actionRow(document, onOpenOptions, onRecheck, "Check again"));
}

/** The controls at the foot of a panel: an optional re-run, then Settings. */
function actionRow(document: Document, onOpenOptions: () => void, onRerun: (() => void) | undefined, rerunLabel: string): HTMLElement {
	const actions = document.createElement("div");
	actions.className = "actions";
	if (onRerun) {
		const again = el(document, "button", rerunLabel) as HTMLButtonElement;
		again.type = "button";
		again.className = "recheck";
		again.title = "Run every check again and refresh the photo results";
		again.addEventListener("click", onRerun);
		actions.append(again);
	}
	const settings = el(document, "button", "Settings") as HTMLButtonElement;
	settings.type = "button";
	settings.className = "settings";
	settings.addEventListener("click", onOpenOptions);
	actions.append(settings);
	return actions;
}

/** The packaged mark, when the extension runtime is present. Absent under test. */
function markURL(): string {
	return runtimeAssetURL("assets/craig-icon-48.png");
}

/** Sprite filenames, one per concern state, exported from mascot-motion under these same names. */
const CONCERN_SPRITE_FILE: Record<ConcernState, string> = {
	low: "craig-low-concern.png",
	caution: "craig-caution.png",
	elevated: "craig-elevated-concern.png",
	high: "craig-high-concern.png",
	"hard-flag": "craig-hard-flag.png",
	incomplete: "craig-incomplete.png"
};

/** The full-body concern sprite, when the extension runtime is present. Absent under test. */
function concernSpriteURL(state: ConcernState): string {
	return runtimeAssetURL(`assets/${CONCERN_SPRITE_FILE[state]}`);
}

/** The loading video, when the extension runtime is present. Absent under test. */
function videoURL(): string {
	return runtimeAssetURL("assets/peek-search.mp4");
}

function runtimeAssetURL(path: string): string {
	try {
		return typeof chrome !== "undefined" && chrome.runtime && chrome.runtime.getURL
			? chrome.runtime.getURL(path)
			: "";
	} catch {
		return "";
	}
}

function panelFor(document: Document): HTMLElement {
	if (loadingRotationTimer !== undefined) {
		clearInterval(loadingRotationTimer);
		loadingRotationTimer = undefined;
	}
	let host = document.getElementById(hostID) as HTMLElement | null;
	if (!host) {
		host = document.createElement("aside");
		host.id = hostID;
		host.setAttribute("aria-live", "polite");
		(document.body || document.documentElement).append(host);
		host.attachShadow({ mode: "open" });
	}
	const shadow = host.shadowRoot!;
	shadow.replaceChildren(style(document));
	const panel = document.createElement("section");
	panel.className = "panel";
	shadow.append(panel);
	return panel;
}

function renderHeader(panel: HTMLElement, title: string, subtitle: string, spriteURL?: string): void {
	const document = panel.ownerDocument;
	const eyebrow = el(document, "span", "CRAIG");
	eyebrow.className = "eyebrow";
	const mark = document.createElement("div");
	mark.className = spriteURL !== undefined ? "mark sprite" : "mark";
	if (spriteURL) mark.style.backgroundImage = `url("${spriteURL}")`;
	mark.setAttribute("aria-hidden", "true");
	panel.append(mark, eyebrow, el(document, "h2", title), el(document, "p", subtitle));
}

const SVG_NS = "http://www.w3.org/2000/svg";
// The gauge is a 240° ring (a 120° gap centered at the bottom) drawn with two
// stacked <circle> elements and stroke-dasharray, rotated so the dash pattern
// starts at the lower-left instead of a native circle's 3-o'clock origin.
const GAUGE_CENTER = { x: 90, y: 76 };
const GAUGE_RADIUS = 58;
const GAUGE_SWEEP_DEG = 240;
const GAUGE_ROTATION_DEG = 150;

function scoreGauge(document: Document, score: number, band: string): HTMLElement {
	const wrapper = document.createElement("div");
	wrapper.className = `gauge ${band}`;
	wrapper.setAttribute("role", "meter");
	wrapper.setAttribute("aria-label", `Risk score: ${score} out of 100`);
	wrapper.setAttribute("aria-valuemin", "0");
	wrapper.setAttribute("aria-valuemax", "100");
	wrapper.setAttribute("aria-valuenow", String(score));

	const { x: cx, y: cy } = GAUGE_CENTER;
	const circumference = 2 * Math.PI * GAUGE_RADIUS;
	const trackLength = (circumference * GAUGE_SWEEP_DEG) / 360;
	const progressLength = trackLength * (Math.max(0, Math.min(100, score)) / 100);

	const svg = document.createElementNS(SVG_NS, "svg");
	svg.setAttribute("viewBox", "0 0 180 140");
	svg.setAttribute("class", "gauge-svg");
	svg.setAttribute("aria-hidden", "true");
	svg.append(
		gaugeArc(document, cx, cy, trackLength, circumference, "gauge-track"),
		gaugeArc(document, cx, cy, progressLength, circumference, "gauge-progress")
	);

	const text = document.createElementNS(SVG_NS, "text");
	text.setAttribute("x", String(cx));
	text.setAttribute("y", String(cy));
	text.setAttribute("class", "gauge-value");
	text.setAttribute("text-anchor", "middle");
	text.setAttribute("dominant-baseline", "central");
	text.textContent = String(score);
	svg.append(text);

	wrapper.append(svg);
	return wrapper;
}

function gaugeArc(document: Document, cx: number, cy: number, visibleLength: number, circumference: number, className: string): SVGCircleElement {
	const circle = document.createElementNS(SVG_NS, "circle") as SVGCircleElement;
	circle.setAttribute("cx", String(cx));
	circle.setAttribute("cy", String(cy));
	circle.setAttribute("r", String(GAUGE_RADIUS));
	circle.setAttribute("class", className);
	circle.setAttribute("stroke-dasharray", `${visibleLength} ${circumference - visibleLength}`);
	circle.setAttribute("transform", `rotate(${GAUGE_ROTATION_DEG} ${cx} ${cy})`);
	return circle;
}

type CheckTone = "danger" | "caution" | "positive" | "neutral";

function appendFindings(document: Document, panel: HTMLElement, title: string, findings: Finding[], tone: CheckTone, state: string, open: boolean): void {
	if (findings.length === 0) return;
	appendGroupHeading(document, panel, title, findings.length);
	for (const finding of findings) {
		const { card, body } = checkCase(document, finding.label, state, tone, open);
		if (finding.detail) body.append(el(document, "p", finding.detail));
		appendTextMatch(document, body, finding);
		appendImageMatches(document, body, finding);
		panel.append(card);
	}
}

function appendTextMatch(document: Document, body: HTMLElement, finding: Finding): void {
	const evidence = finding.text_match;
	if (!evidence?.match) return;

	const section = document.createElement("section");
	section.className = "text-match";
	section.append(el(document, "p", "Matched listing text"));
	const excerpt = document.createElement("p");
	excerpt.className = "text-excerpt";
	if (evidence.before) excerpt.append(document.createTextNode(`…${evidence.before} `));
	const match = document.createElement("mark");
	match.textContent = evidence.match;
	excerpt.append(match);
	if (evidence.after) excerpt.append(document.createTextNode(` ${evidence.after}…`));
	section.append(excerpt);
	body.append(section);
}

function appendNames(document: Document, panel: HTMLElement, title: string, names: string[], open: boolean): void {
	if (names.length === 0) return;
	appendGroupHeading(document, panel, title, names.length);
	for (const name of names) {
		const { card, body } = checkCase(document, humanize(name), "Passed", "positive", open);
		body.append(el(document, "p", passedCheckDescription(name)));
		panel.append(card);
	}
}

// Plain-language explanations of what a passed check actually looked for, so
// a green result reads as "here's what we ruled out" rather than an opaque
// rule name. Keyed by rule name from internal/rules/default_rules.yaml;
// custom/marketplace rules outside this set fall back to a generic line.
const PASSED_CHECK_DESCRIPTIONS: Record<string, string> = {
	payment_no_recourse: "No requests for irreversible payment methods like wire transfers, gift cards, or crypto-only payment were found.",
	urgency_pressure: "No high-pressure phrases like \"must go today\" or \"first come, first served\" were found.",
	absentee_landlord: "The lister didn't claim to be out of town, unreachable, or unable to meet in person.",
	contact_evasion_phrases: "No common evasion phrases like \"email only,\" \"no calls,\" or \"contact through the app\" were found.",
	deposit_before_viewing: "No request for a deposit or hold before you've seen the place.",
	prepayment_before_access: "No request to pay before a tour, keys, a lockbox code, or other physical access was found.",
	obfuscated_contact: "Contact details weren't written in a disguised way, like spelled-out digits or spaced-out email addresses.",
	relay_only_contact: "The listing isn't limited to in-app messaging only — a direct contact channel is available.",
	direct_phone_listed: "This listing didn't include a direct phone number in the post.",
	nonstandard_rental_fee: "No hold, reservation, or similarly nonstandard fee was found.",
	high_standard_rental_fee: "No standard rental fee exceeded its caution threshold.",
	rent_price_mismatch: "The page price and any rent advertised in the title did not materially conflict.",
	reverse_image_real_estate: "Cloud Vision found no matching real-estate source in the photos checked.",
	stock_photos: "Cloud Vision found no matching stock-photo source in the photos checked.",
	mls_watermark: "No agency or MLS watermark, like \"Coldwell\" or \"Century 21,\" was detected in the listing photos."
};

function passedCheckDescription(rule: string): string {
	return PASSED_CHECK_DESCRIPTIONS[rule] || "Completed with no matching indicator.";
}

function appendNotEvaluated(document: Document, panel: HTMLElement, entries: NotEvaluated[]): void {
	if (entries.length === 0) return;
	appendGroupHeading(document, panel, "Not evaluated", entries.length);
	for (const entry of entries) {
		const { card, body } = checkCase(document, humanize(entry.rule), "Skipped", "neutral", false);
		body.append(el(document, "p", skipReason(entry.reason)));
		panel.append(card);
	}
}

function appendTrace(document: Document, panel: HTMLElement, events: TraceEvent[]): void {
	const { card: details, body } = checkCase(document, "Verified execution log", `${events.length} events`, "neutral", false);
	details.classList.add("trace");
	const note = el(document, "p", "Events are produced by the extension and its built-in engine. Secrets, listing text, and image URLs are excluded.");
	note.className = "trace-note";
	const output = document.createElement("pre");
	output.textContent = events.length === 0
		? "No execution events were returned. Reload the extension and try again."
		: events.map((event) => `${formatTime(event.timestamp)} ${event.step.padEnd(10)} ${event.message}`).join("\n");
	body.append(note, output);
	panel.append(details);
}

function appendGroupHeading(document: Document, panel: HTMLElement, title: string, count: number): void {
	const heading = el(document, "h3", `${title} (${count})`);
	heading.className = "group-heading";
	panel.append(heading);
}

function checkCase(document: Document, title: string, state: string, tone: CheckTone, open: boolean): { card: HTMLDetailsElement; body: HTMLElement } {
	const card = document.createElement("details");
	card.className = `check-case ${tone}`;
	card.open = open;
	const summary = document.createElement("summary");
	const caseTitle = el(document, "span", title);
	caseTitle.className = "case-title";
	const caseState = el(document, "span", state);
	caseState.className = "case-state";
	summary.append(caseTitle, caseState);
	const body = document.createElement("div");
	body.className = "case-body";
	card.append(summary, body);
	return { card, body };
}

function appendImageMatches(document: Document, body: HTMLElement, finding: Finding): void {
	for (const match of finding.image_matches ?? []) {
		const listingURL = safeRemoteURL(match.listing_image_url);
		const sourcePageURL = safeRemoteURL(match.source_page_url);
		if (!listingURL || !sourcePageURL) continue;

		const evidence = document.createElement("section");
		evidence.className = "image-match";
		evidence.append(el(document, "p", "Reverse-image evidence"));
		const pair = document.createElement("div");
		pair.className = "image-pair";
		pair.append(imageFigure(document, listingURL, "Listing image", "Image submitted for analysis"));

		const sourceImageURL = safeRemoteURL(match.source_image_url);
		if (sourceImageURL) {
			pair.append(imageFigure(document, sourceImageURL, "Matching image", "Image returned by reverse-image search"));
		} else {
			const unavailable = el(document, "p", "A matching image preview was not supplied by Vision.");
			unavailable.className = "image-unavailable";
			pair.append(unavailable);
		}
		evidence.append(pair);
		const source = document.createElement("a");
		source.className = "match-source";
		source.href = sourcePageURL;
		source.target = "_blank";
		source.rel = "noopener noreferrer";
		source.textContent = sourcePageURL;
		evidence.append(el(document, "p", "Detected at:"), source);
		body.append(evidence);
	}
}

function imageFigure(document: Document, url: string, label: string, alt: string): HTMLElement {
	const figure = document.createElement("figure");
	const image = document.createElement("img");
	image.src = url;
	image.alt = alt;
	image.loading = "lazy";
	image.referrerPolicy = "no-referrer";
	figure.append(image, el(document, "figcaption", label));
	return figure;
}

function safeRemoteURL(value: string | undefined): string | undefined {
	if (!value) return undefined;
	try {
		const parsed = new URL(value);
		return parsed.protocol === "https:" || parsed.protocol === "http:" ? parsed.href : undefined;
	} catch {
		return undefined;
	}
}

function formatTime(value: string): string {
	const time = new Date(value);
	if (Number.isNaN(time.valueOf())) return "--:--:--.---";
	return time.toLocaleTimeString([], { hour12: false, hour: "2-digit", minute: "2-digit", second: "2-digit" }) + `.${String(time.getMilliseconds()).padStart(3, "0")}`;
}

function el(document: Document, tag: string, text: string): HTMLElement {
	const element = document.createElement(tag);
	element.textContent = text;
	return element;
}

function humanize(value: string): string {
	return value.replace(/_/g, " ").replace(/\b\w/g, (letter) => letter.toUpperCase());
}

function skipReason(reason: string): string {
	const labels: Record<string, string> = {
		no_api_key: "Needs a Google Vision API key",
		vision_budget_exhausted: "Monthly Vision budget exhausted",
		no_images: "Listing has no images",
        partial_images: "Some photos could not be checked; this check is incomplete",
		missing_field: "Listing did not include the needed field",
		market_rent_inputs_missing: "Needs a listed monthly USD price and a studio-to-four-bedroom count",
		market_rent_benchmark_unavailable: "No bundled HUD benchmark is available for this ZIP and bedroom count",
		rent_price_inputs_missing: "Needs a listed page price and a title with a dollar amount",
		provider_error: "Provider could not complete the check"
	};
	return labels[reason] || humanize(reason);
}

function style(document: Document): HTMLStyleElement {
	const element = document.createElement("style");
	const mark = markURL();
	element.textContent = `
    :host { all: initial; }
    .panel { box-sizing: border-box; position: fixed; z-index: 2147483647; top: 20px; right: 20px; width: min(430px, calc(100vw - 32px)); max-height: calc(100vh - 40px); overflow: auto; padding: 18px; border: 2px solid #0c0e0d; background: #f8faf7; color: #0c0e0d; box-shadow: 8px 8px 0 rgba(12, 14, 13, .14); font: 15px/1.55 system-ui, -apple-system, "Segoe UI", sans-serif; }
    .mark { float: right; width: 26px; height: 26px; margin: 0 0 8px 12px; background-repeat: no-repeat; background-size: 26px 26px; background-position: center; image-rendering: pixelated; ${mark ? `background-image: url("${mark}");` : ""} }
    .mark.sprite { width: 64px; height: 91px; background-size: 64px 91px; }
    .eyebrow { display: block; margin-bottom: 9px; color: #313631; font: 700 10px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .14em; }
    h2 { margin: 0; font-size: 22px; line-height: 1.15; letter-spacing: -.02em; } p { margin: 8px 0 12px; color: #313631; }
    .coverage { margin: 13px 0; padding: 9px 0; border-top: 2px solid #0c0e0d; border-bottom: 1px solid #c9cec8; color: #0c0e0d; font: 600 12px/1.45 ui-monospace, SFMono-Regular, Menlo, monospace; }
    .loading-visual { display: flex; flex-direction: column; align-items: center; margin: 4px 0 6px; }
    .loading-video { width: 160px; height: 229px; object-fit: contain; }
    .loading-status { min-height: 34px; margin: 6px 0 0; color: #0c0e0d; font: 700 12px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .02em; text-align: center; }
    .gauge { display: flex; justify-content: center; margin: 4px 0 10px; color: #0c0e0d; } .gauge-svg { width: 168px; height: 130px; } .gauge-track { fill: none; stroke: #c9cec8; stroke-width: 12; } .gauge-progress { fill: none; stroke: currentColor; stroke-width: 12; } .gauge-value { fill: currentColor; font: 800 34px/1 ui-monospace, SFMono-Regular, Menlo, monospace; } .gauge.low { color: #2f8a35; } .gauge.caution { color: #9a6714; } .gauge.elevated { color: #b2571f; } .gauge.high { color: #b23a38; }
    .group-heading { margin: 18px 0 9px; padding-top: 11px; border-top: 2px solid #0c0e0d; color: #0c0e0d; font: 700 11px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .1em; text-transform: uppercase; }
    .check-case { margin-top: 8px; border: 1px solid #0c0e0d; background: #f8faf7; } .check-case summary { display: flex; align-items: center; gap: 10px; padding: 10px 11px; cursor: pointer; font-weight: 650; list-style: none; } .check-case summary::-webkit-details-marker { display: none; } .check-case summary:focus-visible { outline: 2px solid #0c0e0d; outline-offset: 2px; } .check-case[open] summary { border-bottom: 1px solid #0c0e0d; } .case-title { min-width: 0; } .case-state { margin-left: auto; padding: 2px 6px; border: 1px solid #0c0e0d; font: 700 10px/1.5 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .06em; text-align: right; text-transform: uppercase; } .check-case.danger { border-color: #b23a38; } .check-case.danger[open] summary { border-bottom-color: #b23a38; } .check-case.danger .case-state { background: #b23a38; color: #f8faf7; border-color: #b23a38; } .check-case.caution { border-color: #9a6714; } .check-case.caution[open] summary { border-bottom-color: #9a6714; } .check-case.caution .case-state { background: #9a6714; color: #f8faf7; border-color: #9a6714; } .check-case.positive { border-color: #2f8a35; } .check-case.positive[open] summary { border-bottom-color: #2f8a35; } .check-case.positive .case-state { border-style: dashed; border-color: #2f8a35; color: #2f8a35; }
    .case-body { padding: 11px; } .case-body > p { margin: 0; color: #313631; font-size: 13px; } .text-match, .image-match { margin-top: 11px; padding-top: 11px; border-top: 1px solid #c9cec8; } .text-match > p, .image-match > p { margin: 0 0 7px; font: 700 11px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .06em; text-transform: uppercase; } .text-excerpt { margin: 0 !important; color: #313631; font-size: 13px; line-height: 1.6; overflow-wrap: anywhere; } mark { padding: 1px 4px; background: #0c0e0d; color: #f8faf7; font-weight: 700; } .image-pair { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; } figure { min-width: 0; margin: 0; } figure img { display: block; width: 100%; aspect-ratio: 4 / 3; border: 1px solid #0c0e0d; background: #c9cec8; object-fit: cover; } figcaption { margin-top: 5px; color: #313631; font: 11px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace; } .image-unavailable { display: flex; align-items: center; box-sizing: border-box; min-height: 96px; margin: 0; padding: 8px; border: 1px dashed #0c0e0d; color: #313631; font-size: 11px; } .match-source { display: block; overflow-wrap: anywhere; color: #0c0e0d; font-size: 12px; text-decoration: underline; }
    .trace { clear: both; } .trace-note { margin: 0 0 8px !important; font-size: 12px !important; } pre { max-height: 230px; overflow: auto; margin: 8px 0 0; padding: 11px; background: #0c0e0d; color: #f8faf7; font: 11px/1.55 ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; white-space: pre-wrap; word-break: break-word; }
    .actions { display: flex; gap: 8px; margin-top: 16px; padding-top: 14px; border-top: 2px solid #0c0e0d; } button { padding: 9px 12px; border: 2px solid #0c0e0d; background: #f8faf7; color: #0c0e0d; cursor: pointer; font: 700 12px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .06em; text-transform: uppercase; } button:hover { background: #0c0e0d; color: #f8faf7; } button:focus-visible { outline: 2px solid #0c0e0d; outline-offset: 2px; } .recheck { background: #0c0e0d; color: #f8faf7; } .recheck:hover { background: #f8faf7; color: #0c0e0d; } .settings { margin-left: auto; }
  `;
	return element;
}
