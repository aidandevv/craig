import assert from "node:assert/strict";
import test from "node:test";
import type { WorkerRequest } from "../shared/types";

test("worker protects storage, rule writes, and billable refreshes", async () => {
  type Reply = { ok: boolean; error?: string; autoRun?: boolean; [key: string]: unknown };
  type Listener = (request: WorkerRequest, sender: chrome.runtime.MessageSender, reply: (response: Reply) => void) => boolean | void;
  let listener!: Listener;
  let clicked!: (tab: chrome.tabs.Tab) => void;
  let removed!: (id: number) => void;
  let grant: { refreshToken: string } | undefined;
  let access = "";
  const stored: Record<string, unknown> = { visionApiKey: "test-key", monthlyCap: 999, maxPhotos: 4, autoRun: true };
  const requests: Record<string, unknown>[] = [];
  let failNext = false;
  const originalChrome = globalThis.chrome;
  const originalNow = Date.now;
  let now = 100_000;
  Date.now = () => now;
  globalThis.chrome = {
    action: { onClicked: { addListener: (fn: typeof clicked) => { clicked = fn; } } },
    tabs: { onRemoved: { addListener: (fn: typeof removed) => { removed = fn; } }, sendMessage: async (_id: number, message: typeof grant) => { grant = message; } },
    runtime: { id: "test-extension", getURL: (path: string) => `chrome-extension://test-extension/${path}`, onMessage: { addListener: (fn: Listener) => { listener = fn; } }, lastError: undefined },
    storage: { local: {
      setAccessLevel: async ({ accessLevel }: { accessLevel: string }) => { access = accessLevel; },
      get: (keys: unknown, callback: (items: Record<string, unknown>) => void) => {
        if (keys && typeof keys === "object" && !Array.isArray(keys)) callback({ ...(keys as Record<string, unknown>), ...stored });
        else if (keys === null) callback({ ...stored });
        else callback(Object.fromEntries((Array.isArray(keys) ? keys : [keys as string]).filter(k => k in stored).map(k => [k, stored[k]])));
      },
      set: (items: Record<string, unknown>, callback: () => void) => { Object.assign(stored, items); callback(); },
      remove: (_keys: unknown, callback: () => void) => callback()
    } }
  } as unknown as typeof chrome;
  globalThis.craigAnalyze = async json => {
    if (failNext) throw new Error("engine exploded");
    requests.push(JSON.parse(json));
    return JSON.stringify({ risk_score: 0, not_evaluated: [], trace: [] });
  };
  globalThis.craigPrepareRules = async json => json;
  try {
    await import("../background/worker");
    const listing = { marketplace: "craigslist", listing_url: "https://sfbay.craigslist.org/apa/1.html", title: "Studio", contact: {} };
    const sender = { id: chrome.runtime.id, frameId: 0, url: listing.listing_url, tab: { id: 1, url: listing.listing_url } } as chrome.runtime.MessageSender;
    const options = { id: chrome.runtime.id, url: chrome.runtime.getURL("dist/options/index.html") };
    const send = (message: WorkerRequest, from = sender) => new Promise<Reply>(resolve => listener(message, from, resolve));
    const request: WorkerRequest = { type: "ANALYZE_LISTING", listing };
    assert.equal(access, "TRUSTED_CONTEXTS");
    assert.deepEqual(await send({ type: "AUTO_RUN_GET" }), { ok: true, autoRun: true });
    assert.equal((await send({ type: "RULES_PUT", rules: {} })).ok, false);
    assert.equal(stored["craig:rules"], undefined);
    assert.equal((await send({ type: "RULES_PUT", rules: { valid: "mock" } }, options)).ok, true);
    for (const bad of [{ ...sender, id: "other" }, { ...sender, frameId: 1 }, { ...sender, url: "https://evil.example/" }]) {
      assert.equal((await send(request, bad)).ok, false);
    }
    assert.equal((await send({ ...request, listing: { ...listing, listing_url: "https://sfbay.craigslist.org/apa/2.html" } })).ok, false);
    assert.equal((await send({ ...request, listing: { ...listing, captions: ["a".repeat(4097)] } })).ok, false);
    assert.equal((await send({ ...request, force: true })).ok, false);
    assert.equal((await send(request)).ok, true);
    assert.equal(requests[0].fresh, false);
    assert.equal(requests[0].cache_only, false);
    assert.equal((requests[0].vision as Record<string, unknown>).api_key, "test-key");
    assert.equal((await send(request)).ok, false);
    now += 10_001;
    clicked(sender.tab!);
    assert.equal((await send({ ...request, force: true, refreshToken: grant!.refreshToken })).ok, true);
    assert.equal(requests[1].fresh, true);
    now += 10_001;
    assert.equal((await send({ ...request, force: true, refreshToken: grant!.refreshToken })).ok, false);
    clicked(sender.tab!);
    now += 15_001;
    assert.equal((await send({ ...request, force: true, refreshToken: grant!.refreshToken })).ok, false);
    stored.autoRun = false;
    assert.equal((await send(request)).ok, true);
    assert.equal(requests[2].cache_only, true);
    now += 10_001;
    failNext = true;
    const failed = await send(request);
    assert.equal(failed.ok, false);
    assert.match(failed.error!, /engine exploded/);
    removed(1);
  } finally {
    Date.now = originalNow;
    globalThis.chrome = originalChrome;
    globalThis.craigAnalyze = undefined;
    globalThis.craigPrepareRules = undefined;
  }
});
