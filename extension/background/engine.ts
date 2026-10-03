import "../generated/wasm_exec.js";

let ready: Promise<void> | undefined;
const defaultBytes = async (): Promise<BufferSource> => {
  const response = await fetch(chrome.runtime.getURL("dist/engine.wasm"), { signal: AbortSignal.timeout(10_000) });
  if (!response.ok) throw new Error("Engine could not be loaded.");
  return response.arrayBuffer();
};

function clearBridge(): void {
  globalThis.craigAnalyze = undefined;
  globalThis.craigPrepareRules = undefined;
  globalThis.craigDefaultRules = undefined;
  globalThis.craigRuleSchema = undefined;
  globalThis.craigEngineReady = undefined;
}

// Failed startup and runtime exit invalidate the bridge so later calls can retry.
export function loadEngine(fetchBytes: () => Promise<BufferSource> = defaultBytes): Promise<void> {
  if (typeof globalThis.craigAnalyze === "function" && !ready) return Promise.resolve();
  ready ??= (async () => {
    const go = new Go();
    const { instance } = await WebAssembly.instantiate(await fetchBytes(), go.importObject);
    const pending = new Set<(error: Error) => void>();
    const race = <T extends unknown[]>(fn: (...args: T) => Promise<string>) =>
      (...args: T): Promise<string> => new Promise((resolve, reject) => {
        pending.add(reject);
        void Promise.resolve().then(() => fn(...args)).then(
          value => { pending.delete(reject); resolve(value); },
          error => { pending.delete(reject); reject(error); }
        );
      });
    await new Promise<void>((resolve, reject) => {
      const exited = () => {
        clearTimeout(timer);
        pending.forEach(reject => reject(new Error("Engine stopped unexpectedly.")));
        pending.clear();
        clearBridge();
        ready = undefined;
        reject(new Error("Engine stopped unexpectedly."));
      };
      const timer = setTimeout(exited, 10_000);
      globalThis.craigEngineReady = () => {
        if (!globalThis.craigAnalyze || !globalThis.craigPrepareRules || !globalThis.craigDefaultRules || !globalThis.craigRuleSchema) return exited();
        globalThis.craigAnalyze = race(globalThis.craigAnalyze);
        globalThis.craigPrepareRules = race(globalThis.craigPrepareRules);
        globalThis.craigDefaultRules = race(globalThis.craigDefaultRules);
        globalThis.craigRuleSchema = race(globalThis.craigRuleSchema);
        clearTimeout(timer);
        resolve();
      };
      try { void go.run(instance).then(exited, exited); } catch { exited(); }
    });
  })().catch(() => {
    clearBridge();
    ready = undefined;
    throw new Error("Engine could not be initialized.");
  });
  return ready;
}
