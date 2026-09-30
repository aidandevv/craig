import assert from "node:assert/strict";
import test from "node:test";
import { loadEngine } from "../background/engine";

test("engine startup failure retries and runtime exit rejects pending calls", async () => {
  const originalGo = Go;
  let fail = true;
  let stop!: (reason: Error) => void;
  const host = globalThis as unknown as { Go: typeof Go };
  host.Go = class {
    importObject = {};
    run(): Promise<void> {
      if (fail) return Promise.reject(new Error("startup failed"));
      globalThis.craigAnalyze = () => new Promise<string>(() => {});
      globalThis.craigPrepareRules = async () => "{}";
      globalThis.craigDefaultRules = async () => "{}";
      globalThis.craigRuleSchema = async () => "{}";
      globalThis.craigEngineReady!();
      return new Promise<void>((_, reject) => { stop = reject; });
    }
  };
  const bytes = async () => new Uint8Array([0, 97, 115, 109, 1, 0, 0, 0]);
  try {
    await assert.rejects(loadEngine(bytes), /could not be initialized/);
    fail = false;
    await loadEngine(bytes);
    const pending = globalThis.craigAnalyze!("{}");
    stop(new Error("runtime failed"));
    await assert.rejects(pending, /stopped unexpectedly/);
    assert.equal(globalThis.craigAnalyze, undefined);
  } finally { host.Go = originalGo; }
});
