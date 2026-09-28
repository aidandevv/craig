import assert from "node:assert/strict";
import test from "node:test";
import type { WorkerRequest, WorkerResponse } from "../shared/types";

test("worker forwards fresh intent, omits an empty key, and reports engine errors", async () => {
  type Listener = (request: WorkerRequest, sender: unknown, reply: (response: WorkerResponse) => void) => boolean | void;
  let listener: Listener | undefined;
  const stored: Record<string, unknown> = {
    visionApiKey: "", monthlyCap: 999, maxPhotos: 4,
    daemonUrl: "http://127.0.0.1:8765", daemonId: "", token: "", autoRun: false
  };
  const requests: Record<string, unknown>[] = [];
  let failNext = false;
  const originalChrome = globalThis.chrome;
  globalThis.chrome = {
    action: { onClicked: { addListener: () => {} } },
    runtime: { onMessage: { addListener: (fn: Listener) => { listener = fn; } }, lastError: undefined },
    storage: { local: {
      get: (keys: unknown, callback: (items: Record<string, unknown>) => void) => {
        if (keys && typeof keys === "object" && !Array.isArray(keys)) callback({ ...(keys as Record<string, unknown>), ...stored });
        else if (keys === null) callback({ ...stored });
        else callback(Object.fromEntries((Array.isArray(keys) ? keys : [keys as string]).filter((k) => k in stored).map((k) => [k, stored[k]])));
      },
      set: (items: Record<string, unknown>, callback: () => void) => { Object.assign(stored, items); callback(); },
      remove: (_keys: unknown, callback: () => void) => callback()
    } }
  } as unknown as typeof chrome;
  globalThis.craigAnalyze = async (json) => {
    if (failNext) throw new Error("engine exploded");
    requests.push(JSON.parse(json));
    return JSON.stringify({ risk_score: 0, not_evaluated: [], trace: [] });
  };
  try {
    await import("../background/worker");
    const listing = { marketplace: "craigslist", listing_url: "https://sfbay.craigslist.org/apa/1.html", title: "Studio", contact: {} };
    const send = (force: boolean) => new Promise<WorkerResponse>((resolve) => listener!({ type: "ANALYZE_LISTING", listing, force }, {}, resolve));

    assert.equal((await send(false)).ok, true);
    assert.equal((await send(true)).ok, true);
    assert.equal(requests[0].fresh, false);
    assert.equal(requests[1].fresh, true);
    assert.equal((requests[0].vision as Record<string, unknown>).api_key, undefined);
    assert.equal((requests[0].vision as Record<string, unknown>).max_images, 4);

    failNext = true;
    const failed = await send(false);
    assert.equal(failed.ok, false);
    assert.match(failed.ok ? "" : failed.error, /engine exploded/);
  } finally {
    globalThis.chrome = originalChrome;
    globalThis.craigAnalyze = undefined;
  }
});
