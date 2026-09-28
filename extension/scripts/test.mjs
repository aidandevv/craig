import { build } from "esbuild";
import { cp, mkdir, rm } from "node:fs/promises";
import { spawn } from "node:child_process";

const output = ".test-dist";
await rm(output, { recursive: true, force: true });
await mkdir(output, { recursive: true });
await build({
  entryPoints: ["test/craigslist.test.ts", "test/badge.test.ts", "test/daemon-client.test.ts", "test/rule-model.test.ts", "test/worker.test.ts", "test/evidence-store.test.ts", "test/engine.test.ts"],
  outbase: ".",
  outdir: output,
  bundle: true,
  external: ["jsdom"],
  format: "esm",
  platform: "node",
  target: "node22",
  logLevel: "silent"
});
await cp("test/fixtures", ".test-dist/test/fixtures", { recursive: true });

const files = [".test-dist/test/craigslist.test.js", ".test-dist/test/badge.test.js", ".test-dist/test/daemon-client.test.js", ".test-dist/test/rule-model.test.js", ".test-dist/test/worker.test.js", ".test-dist/test/evidence-store.test.js", ".test-dist/test/engine.test.js"];
const child = spawn(process.execPath, ["--test", ...files], { stdio: "inherit" });
const code = await new Promise((resolve, reject) => {
  child.once("error", reject);
  child.once("exit", resolve);
});
await rm(output, { recursive: true, force: true });
process.exitCode = code ?? 1;
