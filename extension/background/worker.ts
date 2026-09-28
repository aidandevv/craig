import { loadSettings } from "../shared/storage";
import type { Assessment, ListingPayload, TraceEvent, WorkerRequest, WorkerResponse } from "../shared/types";

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
	if (message.type !== "ANALYZE_LISTING") {
		return;
	}
	void analyze(message.listing, Boolean(message.force)).then(sendResponse);
	return true;
});

async function analyze(listing: ListingPayload, force: boolean): Promise<WorkerResponse> {
    // Always ask the daemon to score current rules. It caches provider evidence.
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
	const localTrace = [
		extensionTrace("extension", `Prepared normalized listing with ${listing.images?.length || 0} image URL(s).`),
		extensionTrace("extension", "Sending authenticated request to the local daemon.")
	];
	const settings = await loadSettings();
	if (!settings.token.trim()) {
		const trace = [...localTrace, extensionTrace("extension", "Stopped before request: no daemon token is configured.")];
		writeTrace(trace);
		return { ok: false, error: "No connection code is set. Open Craig settings and paste the code from `craig-extension config token`.", trace };
	}
	const baseURL = settings.daemonUrl.replace(/\/$/, "");
	try {
		const response = await fetch(`${baseURL}/api/analyze${force ? "?fresh=true" : ""}`, {
			method: "POST",
			headers: {
				"Authorization": `Bearer ${settings.token}`,
				"Content-Type": "application/json"
			},
			body: JSON.stringify(listing),
			signal: AbortSignal.timeout(45_000)
		});
		const payload = await response.json().catch(() => ({})) as AnalyzePayload;
		const trace = [...localTrace, ...normalizeTrace(payload.trace)];
		if (!response.ok) {
			trace.push(extensionTrace("extension", `Local daemon returned HTTP ${response.status}.`));
			writeTrace(trace);
			return { ok: false, error: payload.error || `The local daemon returned HTTP ${response.status}.`, trace };
		}
		trace.push(extensionTrace("extension", "Received a completed assessment from the local daemon."));
		writeTrace(trace);
		return { ok: true, assessment: payload, cached: false, trace };
	} catch {
		const trace = [...localTrace, extensionTrace("extension", `Could not reach the local daemon at ${baseURL}.`)];
		writeTrace(trace);
		return { ok: false, error: `Could not reach the Craig helper at ${baseURL}. Start it with \`craig-extension daemon\`, then try again.`, trace };
	}
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
