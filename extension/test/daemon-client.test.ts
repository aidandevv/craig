import assert from "node:assert/strict";
import test from "node:test";

import { daemonCandidates, detectDaemon, validateDaemonURL } from "../options/daemon-client";

test("daemon URL validation permits only a plain local helper URL", () => {
	assert.equal(validateDaemonURL("http://127.0.0.1:8765/"), "http://127.0.0.1:8765");
	assert.equal(validateDaemonURL("http://localhost:9123"), "http://localhost:9123");
	for (const invalid of ["https://127.0.0.1:8765", "http://example.com", "http://user@localhost:8765", "http://localhost:8765/?next=elsewhere"]) {
		assert.equal(validateDaemonURL(invalid), undefined, invalid);
	}
});

test("daemon detection keeps a saved local address first and captures the helper ID", async () => {
	const calls: string[] = [];
	const fetcher: typeof fetch = async (input) => {
		const url = String(input);
		calls.push(url);
		if (url === "http://localhost:9123/healthz") return new Response(JSON.stringify({ status: "ok", daemon_id: "helper-123" }));
		throw new Error("not listening");
	};
	const detected = await detectDaemon("http://localhost:9123", fetcher);
	assert.deepEqual(detected, { url: "http://localhost:9123", daemonID: "helper-123" });
	assert.deepEqual(calls, ["http://localhost:9123/healthz"]);
	assert.deepEqual(daemonCandidates("not a URL"), ["http://127.0.0.1:8765", "http://localhost:8765"]);
});
