import { build } from "esbuild";
import { access, cp, mkdir, rm } from "node:fs/promises";

const dist = new URL("../dist/", import.meta.url);
await rm(dist, { recursive: true, force: true });
await mkdir(dist, { recursive: true });

await build({
  entryPoints: ["background/worker.ts", "content/main.ts", "options/options.ts"],
  outbase: ".",
  outdir: "dist",
  bundle: true,
  format: "iife",
  target: "chrome120",
  sourcemap: true,
  logLevel: "info"
});

await cp("options/index.html", "dist/options/index.html");

try {
  await access("generated/engine.wasm");
} catch {
  throw new Error("generated/engine.wasm is missing. Run `make wasm` from the repository root first.");
}
await cp("generated/engine.wasm", "dist/engine.wasm");
