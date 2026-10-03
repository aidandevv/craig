import { boundedPayload, contentSender, craigslistURL, listingFromSender, optionsSender } from "./security";
import { loadSettings } from "../shared/storage";
import { loadEngine } from "./engine";
import { chromeStorageArea, createEvidenceStore } from "./evidence-store";
import type { Assessment, ListingPayload, TraceEvent, WorkerRequest, WorkerResponse } from "../shared/types";

// The engine reads globalThis.craigStore once at startup, so it must be
// installed before any listener or loadEngine call.
const store = createEvidenceStore(chromeStorageArea(chrome.storage.local));
globalThis.craigStore = store;

// Restrict secrets and the usage ledger before processing any requests.
const storageReady = chrome.storage.local.setAccessLevel({ accessLevel: "TRUSTED_CONTEXTS" }).then(() => true, () => false);
const refreshes = new Map<number, { token: string; url: string; expires: number }>();
const lastAnalysis = new Map<number, number>();

const inFlight = new Map<string, Promise<WorkerResponse>>();

interface AnalyzePayload extends Assessment {
	trace?: TraceEvent[];
	error?: string;
}

chrome.action.onClicked.addListener((tab) => {
  const url = craigslistURL(tab.url);
  if (tab.id === undefined || !url) return;
  const token = crypto.randomUUID();
  refreshes.set(tab.id, { token, url, expires: Date.now() + 15_000 });
  void chrome.tabs.sendMessage(tab.id, { type: "ANALYZE_CURRENT_LISTING", force: true, refreshToken: token }, { frameId: 0 }).catch(() => {
    refreshes.delete(tab.id!);
  });
});
chrome.tabs.onRemoved.addListener((id) => { refreshes.delete(id); lastAnalysis.delete(id); });

chrome.runtime.onMessage.addListener((message: WorkerRequest, sender, sendResponse) => {
  const fromContent = contentSender(sender);
  const fromOptions = optionsSender(sender);
  if (!message || typeof message !== "object" || typeof message.type !== "string" || !boundedPayload(message) ||
      (!fromContent && !fromOptions) ||
      (message.type.startsWith("RULES_") || message.type === "USAGE_GET") && !fromOptions) {
    sendResponse({ ok: false, error: "Request is not authorized.", trace: [] });
    return;
  }
  if (message.type === "AUTO_RUN_GET") {
    void storageReady.then(async (ready) => {
      if (!ready) return sendResponse({ ok: false, autoRun: false });
      const { autoRun } = await loadSettings();
      sendResponse({ ok: true, autoRun });
    }).catch(() => sendResponse({ ok: false, autoRun: false }));
    return true;
  }
	if (message.type === "OPEN_OPTIONS") {
		chrome.runtime.openOptionsPage();
		sendResponse({ ok: true });
		return;
	}
	if (message.type === "RULES_GET") {
		void (async () => {
			await requireStorage();
			const stored = await loadStoredRules();
			if (stored) return stored;
			await loadEngine();
			return JSON.parse(await globalThis.craigDefaultRules!());
		})().then((rules) => sendResponse({ ok: true, rules }), (e) => sendResponse({ ok: false, error: String(e) }));
		return true;
	}
	if (message.type === "RULES_PUT") {
		void (async () => {
			await requireStorage();
			await loadEngine();
			const prepared = JSON.parse(await globalThis.craigPrepareRules!(JSON.stringify(message.rules)));
			await chromeStorageArea(chrome.storage.local).set({ "craig:rules": prepared });
			return prepared;
		})().then((rules) => sendResponse({ ok: true, rules }), (e) => sendResponse({ ok: false, error: e instanceof Error ? e.message : String(e) }));
		return true;
	}
	if (message.type === "RULES_SCHEMA") {
		void requireStorage().then(() => loadEngine()).then(() => globalThis.craigRuleSchema!()).then(
			(schema) => sendResponse({ ok: true, schema: JSON.parse(schema) }), (e) => sendResponse({ ok: false, error: String(e) }));
		return true;
	}
	if (message.type === "USAGE_GET") {
		void requireStorage().then(() => Promise.all([store.unitsUsed("web_detection"), store.unitsUsed("text_detection")])).then(
			([web, text]) => sendResponse({ ok: true, usage: { web_detection: web, text_detection: text } }),
			(e) => sendResponse({ ok: false, error: String(e) }));
		return true;
	}
	if (message.type !== "ANALYZE_LISTING") {
		return;
	}
	if (!fromContent || !listingFromSender(message.listing, sender)) {
    sendResponse({ ok: false, error: "Invalid listing request.", trace: [] });
    return;
  }
  const tabID = sender.tab!.id!;
  const grant = refreshes.get(tabID);
  const force = message.force === true;
  if (force && (!grant || grant.token !== message.refreshToken || grant.url !== craigslistURL(sender.url) || grant.expires < Date.now())) {
    sendResponse({ ok: false, error: "Use the extension toolbar to refresh photos.", trace: [] });
    return;
  }
  if (force) refreshes.delete(tabID);
  const now = Date.now();
  if (now - (lastAnalysis.get(tabID) ?? 0) < 10_000 || inFlight.size >= 4) {
    sendResponse({ ok: false, error: "Please wait before running another analysis.", trace: [] });
    return;
  }
  lastAnalysis.set(tabID, now);
  void analyze(message.listing, force).then(sendResponse, () => sendResponse({ ok: false, error: "Analysis failed.", trace: [] }));
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
		await requireStorage();
		const [settings, rules] = await Promise.all([loadSettings(), loadStoredRules(), loadEngine()]);
		localTrace.push(extensionTrace("extension", settings.visionApiKey ? "Running Craig with photo checks." : "Running Craig without photo checks (no Vision key)."));
		const raw = await globalThis.craigAnalyze!(JSON.stringify({
			listing,
			rules,
			fresh: force,
            cache_only: !force && !settings.autoRun,
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
	if (stored && !boundedPayload(stored)) throw new Error("Saved rules exceed size limits.");
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

async function requireStorage(): Promise<void> {
  if (!await storageReady) throw new Error("Secure storage could not be initialized.");
}
