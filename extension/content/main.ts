import { extractListing } from "./craigslist";
import { renderAssessment, renderError, renderLoading } from "./badge";
import { loadSettings } from "../shared/storage";
import type { TraceEvent, WorkerResponse } from "../shared/types";

chrome.runtime.onMessage.addListener((message: { type?: string; force?: boolean }) => {
	if (message.type === "ANALYZE_CURRENT_LISTING") {
		void analyze(Boolean(message.force));
	}
});

void loadSettings().then((settings) => {
	if (settings.autoRun) {
		void analyze(false);
	}
});

async function analyze(force: boolean): Promise<void> {
	try {
		const listing = extractListing(document);
		renderLoading(document);
		const result = await chrome.runtime.sendMessage({ type: "ANALYZE_LISTING", listing, force }) as WorkerResponse;
		if (result.ok) {
			renderAssessment(document, result.assessment, result.cached, openOptions, result.trace, () => void analyze(true));
			return;
		}
		renderError(document, result.error, openOptions, result.trace, () => void analyze(true));
	} catch (error) {
		const message = error instanceof Error ? error.message : "Unable to extract this listing.";
		renderError(document, message, openOptions, [{ timestamp: new Date().toISOString(), step: "extension", message }]);
	}
}

function openOptions(): void {
	void chrome.runtime.sendMessage({ type: "OPEN_OPTIONS" });
}
