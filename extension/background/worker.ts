import { loadSettings } from "../shared/storage";
import { loadEngine } from "./engine";
import { chromeStorageArea, createEvidenceStore } from "./evidence-store";
import type { Assessment, ListingPayload, TraceEvent, WorkerRequest, WorkerResponse } from "../shared/types";

// The engine reads globalThis.craigStore once at startup, so it must be
// installed before any listener or loadEngine call.
const store = createEvidenceStore(chromeStorageArea(chrome.storage.local));
globalThis.craigStore = store;

const inFlight = new Map<string, Promise<WorkerResponse>>();

interface AnalyzePayload extends Assessment {
	trace?: TraceEvent[];
	error?: string;
}

chrome.action.onClicked.addListener((tab) => {
  if (tab.id === undefined) {
    return;
  }
	void chrome.tabs.sendMessage(tab.id, { type: "ANALYZE_CURRENT_LISTING", force: true }).catch(() => {
		// The active tab may not be a Craigslist listing. There is no page surface
		// to render into in that case, so intentionally leave it alone.
	});
});

chrome.runtime.onMessage.addListener((message: WorkerRequest, _sender, sendResponse) => {
	if (message.type === "OPEN_OPTIONS") {
		chrome.runtime.openOptionsPage();
		sendResponse({ ok: true });
		return;
	}
	if (message.type === "RULES_GET") {
		void (async () => {
			const stored = await loadStoredRules();
			if (stored) return stored;
			await loadEngine();
			return JSON.parse(await globalThis.craigDefaultRules!());
		})().then((rules) => sendResponse({ ok: true, rules }), (e) => sendResponse({ ok: false, error: String(e) }));
		return true;
	}
	if (message.type === "RULES_PUT") {
		void (async () => {
			await loadEngine();
			const prepared = JSON.parse(await globalThis.craigPrepareRules!(JSON.stringify(message.rules)));
			await chromeStorageArea(chrome.storage.local).set({ "craig:rules": prepared });
			return prepared;
		})().then((rules) => sendResponse({ ok: true, rules }), (e) => sendResponse({ ok: false, error: e instanceof Error ? e.message : String(e) }));
		return true;
	}
	if (message.type === "RULES_SCHEMA") {
		void loadEngine().then(() => globalThis.craigRuleSchema!()).then(
			(schema) => sendResponse({ ok: true, schema: JSON.parse(schema) }), (e) => sendResponse({ ok: false, error: String(e) }));
		return true;
	}
	if (message.type === "USAGE_GET") {
		void Promise.all([store.unitsUsed("web_detection"), store.unitsUsed("text_detection")]).then(
			([web, text]) => sendResponse({ ok: true, usage: { web_detection: web, text_detection: text } }),
			(e) => sendResponse({ ok: false, error: String(e) }));
		return true;
	}
	if (message.type !== "ANALYZE_LISTING") {
		return;
	}
	void analyze(message.listing, Boolean(message.force)).then(sendResponse);
	return true;
});

async function analyze(listing: ListingPayload, force: boolean): Promise<WorkerResponse> {
    // Always score with current rules; the engine caches provider evidence.
    const cacheKey = JSON.stringify({listing, force});

	const current = inFlight.get(cacheKey);
	if (current) {
		return current;
	}
	const request = requestAssessment(listing, force).finally(() => inFlight.delete(cacheKey));
	inFlight.set(cacheKey, request);
	const response = await request;
	return response;
}

async function requestAssessment(listing: ListingPayload, force: boolean): Promise<WorkerResponse> {
	const localTrace = [extensionTrace("extension", `Prepared normalized listing with ${listing.images?.length || 0} image URL(s).`)];
	try {
		const [settings, rules] = await Promise.all([loadSettings(), loadStoredRules(), loadEngine()]);
		localTrace.push(extensionTrace("extension", settings.visionApiKey ? "Running Craig with photo checks." : "Running Craig without photo checks (no Vision key)."));
		const raw = await globalThis.craigAnalyze!(JSON.stringify({
			listing,
			rules,
			fresh: force,
			vision: { api_key: settings.visionApiKey || undefined, monthly_cap: settings.monthlyCap, max_images: settings.maxPhotos }
		}));
		const payload = JSON.parse(raw) as AnalyzePayload;
		const trace = [...localTrace, ...normalizeTrace(payload.trace)];
		writeTrace(trace);
		void store.prune().catch(() => undefined);
		return { ok: true, assessment: payload, cached: false, trace };
	} catch (error) {
		const trace = [...localTrace, extensionTrace("extension", "Analysis stopped before completion.")];
		writeTrace(trace);
		return { ok: false, error: error instanceof Error ? error.message : "Craig could not analyze this listing.", trace };
	}
}

async function loadStoredRules(): Promise<unknown | undefined> {
	const stored = (await chromeStorageArea(chrome.storage.local).get("craig:rules"))["craig:rules"];
	return stored && typeof stored === "object" ? stored : undefined;
}

function extensionTrace(step: string, message: string): TraceEvent {
	return { timestamp: new Date().toISOString(), step, message };
}

function normalizeTrace(value: unknown): TraceEvent[] {
	if (!Array.isArray(value)) return [];
	return value.filter((event): event is TraceEvent =>
		typeof event === "object" && event !== null &&
		typeof (event as TraceEvent).timestamp === "string" &&
		typeof (event as TraceEvent).step === "string" &&
		typeof (event as TraceEvent).message === "string"
	);
}

function writeTrace(trace: TraceEvent[]): void {
	for (const event of trace) {
		console.debug(`[Craig] ${event.step}: ${event.message}`);
	}
}
