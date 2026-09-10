import { build } from "esbuild";
import { cp, mkdir, rm } from "node:fs/promises";

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
