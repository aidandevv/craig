import assert from "node:assert/strict";
import test from "node:test";
import type { WorkerRequest } from "../shared/types";

test("worker fails closed if trusted storage cannot be established", async () => {
  let listener!: (message: WorkerRequest, sender: chrome.runtime.MessageSender, reply: (response: { ok: boolean; error: string }) => void) => void;
  let reads = 0;
  const original = globalThis.chrome;
  globalThis.chrome = {
    action: { onClicked: { addListener() {} } },
    tabs: { onRemoved: { addListener() {} } },
    runtime: { id: "test-extension", getURL: (path: string) => `chrome-extension://test-extension/${path}`, onMessage: { addListener: (fn: typeof listener) => { listener = fn; } } },
    storage: { local: { setAccessLevel: async () => { throw new Error("unavailable"); }, get: () => { reads++; } } }
  } as unknown as typeof chrome;
  try {
    await import("../background/worker");
    const sender = { id: chrome.runtime.id, url: chrome.runtime.getURL("dist/options/index.html") };
    const response = await new Promise<{ ok: boolean; error: string }>(resolve => listener({ type: "RULES_GET" }, sender, resolve));
    assert.equal(response.ok, false);
    assert.match(response.error, /Secure storage/);
    assert.equal(reads, 0);
  } finally { globalThis.chrome = original; }
});
