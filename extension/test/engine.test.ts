import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createEvidenceStore, memoryStorageArea } from "../background/evidence-store";
import { loadEngine } from "../background/engine";

const generated = new URL("../../generated/", import.meta.url);

test("loadEngine is memoized across concurrent callers and scores offline", async () => {
  globalThis.craigStore = createEvidenceStore(memoryStorageArea());
  let loads = 0;
  const bytes = async () => { loads++; return new Uint8Array(await readFile(new URL("engine.wasm", generated))); };
  await Promise.all([loadEngine(bytes), loadEngine(bytes)]);
  assert.equal(loads, 1);

  const raw = await globalThis.craigAnalyze!(JSON.stringify({
    listing: {
      marketplace: "craigslist",
      listing_url: "https://sfbay.craigslist.org/apa/1.html",
      title: "Beautiful 2BR - MUST GO TODAY",
      description: "I am out of the country. Send the deposit by western union to hold the unit.",
      contact: { relay_only: true }
    },
    vision: { monthly_cap: 999, max_images: 4 }
  }));
  const assessment = JSON.parse(raw);
  assert.equal(assessment.hard_flagged, true);
  assert.ok(assessment.not_evaluated.some((row: { reason: string }) => row.reason === "no_api_key"));
});

test("craigPrepareRules rejects an empty rule set", async () => {
  await assert.rejects(globalThis.craigPrepareRules!("{}"));
});

test("bridge recovers from host panic and times out unsettled storage safely", async () => {
  const store = globalThis.craigStore!;
  const get = store.getEvidence;
  const reserve = store.reserveUnit;
  const request = JSON.stringify({
    listing: { marketplace: "craigslist", listing_url: "https://sfbay.craigslist.org/apa/1.html", title: "Studio", images: ["https://images.craigslist.org/a.jpg"], contact: {} },
    vision: { api_key: "test-key", monthly_cap: 999, max_images: 4 }
  });
  try {
    store.reserveUnit = () => { throw new Error("host failure"); };
    const failed = JSON.parse(await globalThis.craigAnalyze!(request));
    assert.ok(failed.not_evaluated.some((row: { reason: string }) => row.reason === "provider_error"));
    const settlers: ((value: boolean) => void)[] = [];
    store.reserveUnit = () => new Promise<boolean>(resolve => { settlers.push(resolve); });
    const raw = await Promise.race([
      globalThis.craigAnalyze!(request),
      new Promise<never>((_, reject) => setTimeout(() => reject(new Error("storage wait hung")), 7_000).unref())
    ]);
    assert.ok(JSON.parse(raw).not_evaluated.some((row: { reason: string }) => row.reason === "provider_error"));
    settlers.forEach(settle => settle(false));
    store.reserveUnit = reserve;
    await assert.rejects(globalThis.craigPrepareRules!("{}"));
    assert.ok(JSON.parse(await globalThis.craigDefaultRules!()).rules);
  } finally {
    store.getEvidence = get;
    store.reserveUnit = reserve;
  }
});
