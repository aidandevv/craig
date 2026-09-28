import assert from "node:assert/strict";
import test from "node:test";
import { createEvidenceStore, memoryStorageArea } from "../background/evidence-store";

test("evidence round-trips in the shape the Go engine decodes", async () => {
  const area = memoryStorageArea();
  const store = createEvidenceStore(area);
  await store.putEvidence("h", "web_detection", "{\"responses\":[]}", "2026-09-27T01:00:00Z");
  const raw = await store.getEvidence("h", "web_detection");
  assert.deepEqual(JSON.parse(raw!), { response: "{\"responses\":[]}", checked_at: "2026-09-27T01:00:00Z" });
  assert.equal(await store.getEvidence("missing", "web_detection"), null);
});

test("reserveUnit never exceeds the cap under concurrency", async () => {
  const store = createEvidenceStore(memoryStorageArea(), () => new Date("2026-09-27T12:00:00Z"));
  const grants = await Promise.all(Array.from({ length: 10 }, () => store.reserveUnit("web_detection", 3)));
  assert.equal(grants.filter(Boolean).length, 3);
  assert.equal(await store.unitsUsed("web_detection"), 3);
});

test("budget resets on the UTC month boundary", async () => {
  let now = new Date("2026-09-30T23:59:59Z");
  const store = createEvidenceStore(memoryStorageArea(), () => now);
  assert.equal(await store.reserveUnit("text_detection", 1), true);
  assert.equal(await store.reserveUnit("text_detection", 1), false);
  now = new Date("2026-10-01T00:00:00Z");
  assert.equal(await store.reserveUnit("text_detection", 1), true);
});

test("prune removes only expired evidence", async () => {
  const area = memoryStorageArea();
  const store = createEvidenceStore(area, () => new Date("2026-09-27T12:00:00Z"));
  await store.putEvidence("old", "web_detection", "{}", "2026-09-26T11:00:00Z");
  await store.putEvidence("new", "web_detection", "{}", "2026-09-27T11:00:00Z");
  await area.set({ "craig:ev:junk:web_detection": "not json", unrelated: 1 });
  assert.equal(await store.prune(), 2);
  assert.notEqual(await store.getEvidence("new", "web_detection"), null);
  assert.deepEqual(Object.keys(await area.get(null)).sort(), ["craig:ev:new:web_detection", "unrelated"]);
});
