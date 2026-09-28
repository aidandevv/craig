import "../generated/wasm_exec.js";

let ready: Promise<void> | undefined;

const defaultBytes = async (): Promise<BufferSource> => (await fetch(chrome.runtime.getURL("dist/engine.wasm"))).arrayBuffer();

// loadEngine starts the Go runtime once per service-worker lifetime. Concurrent
// callers share the same instantiation; a failed load is retried next call.
export function loadEngine(fetchBytes: () => Promise<BufferSource> = defaultBytes): Promise<void> {
  if (typeof globalThis.craigAnalyze === "function") return Promise.resolve();
  ready ??= (async () => {
    const go = new Go();
    const { instance } = await WebAssembly.instantiate(await fetchBytes(), go.importObject);
    await new Promise<void>((resolve) => {
      globalThis.craigEngineReady = resolve;
      void go.run(instance);
    });
  })().catch((error) => {
    ready = undefined;
    throw error;
  });
  return ready;
}
