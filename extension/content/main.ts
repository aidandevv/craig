import { extractListing, isListingPage } from "./craigslist";
import { renderAssessment, renderError, renderLoading, renderNotListing } from "./badge";
import type { TraceEvent, WorkerResponse } from "../shared/types";

chrome.runtime.onMessage.addListener((message: { type?: string; force?: boolean; refreshToken?: string }, sender) => {
	if (sender.id !== chrome.runtime.id || sender.url !== chrome.runtime.getURL("dist/background/worker.js")) return;
	if (message.type === "ANALYZE_CURRENT_LISTING") {
		void analyze(Boolean(message.force), message.refreshToken);
	}
});

void chrome.runtime.sendMessage({ type: "AUTO_RUN_GET" }).then((settings) => {
	if (settings?.ok && settings.autoRun && isListingPage(document)) {
		void analyze(false);
	}
}).catch(() => undefined);

async function analyze(force: boolean, refreshToken?: string): Promise<void> {
	if (!isListingPage(document)) {
		renderNotListing(document);
		return;
	}
	try {
		const listing = extractListing(document);
		renderLoading(document);
		const result = await chrome.runtime.sendMessage({ type: "ANALYZE_LISTING", listing, force, refreshToken }) as WorkerResponse;
		if (result.ok) {
			renderAssessment(document, result.assessment, result.cached, openOptions, result.trace, () => void analyze(false));
			return;
		}
		renderError(document, result.error, openOptions, result.trace, () => void analyze(false));
	} catch (error) {
		const message = error instanceof Error ? error.message : "Unable to extract this listing.";
		renderError(document, message, openOptions, [{ timestamp: new Date().toISOString(), step: "extension", message }]);
	}
}

function openOptions(): void {
	void chrome.runtime.sendMessage({ type: "OPEN_OPTIONS" });
}
