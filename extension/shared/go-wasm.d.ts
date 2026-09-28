declare module "*wasm_exec.js";

declare class Go {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

declare var craigAnalyze: ((requestJSON: string) => Promise<string>) | undefined;
declare var craigPrepareRules: ((ruleSetJSON: string) => Promise<string>) | undefined;
declare var craigDefaultRules: (() => Promise<string>) | undefined;
declare var craigRuleSchema: (() => Promise<string>) | undefined;
declare var craigEngineReady: (() => void) | undefined;
declare var craigStore: import("../background/evidence-store").CraigStore | undefined;
