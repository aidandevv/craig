import assert from "node:assert/strict";
import test from "node:test";
import { classifyKeyResponse, testVisionKey } from "../options/vision-key";

const googleError = (status: string, reason?: string) => ({
  error: { status, details: reason ? [{ "@type": "type.googleapis.com/google.rpc.ErrorInfo", reason }] : [] }
});

test("classifyKeyResponse explains each setup mistake", () => {
  const cases: [number, unknown, boolean, RegExp][] = [
    [200, {}, true, /photo checks are on/i],
    [400, googleError("INVALID_ARGUMENT", "API_KEY_INVALID"), false, /doesn't recognize this key/i],
    [403, googleError("PERMISSION_DENIED", "SERVICE_DISABLED"), false, /turn on the cloud vision api/i],
    [403, googleError("PERMISSION_DENIED", "BILLING_DISABLED"), false, /turn on billing/i],
    [403, googleError("PERMISSION_DENIED", "API_KEY_SERVICE_BLOCKED"), false, /restricted to other apis/i],
    [429, googleError("RESOURCE_EXHAUSTED"), false, /try again in a minute/i],
    [500, {}, false, /HTTP 500/]
  ];
  for (const [status, body, ok, message] of cases) {
    const result = classifyKeyResponse(status, body);
    assert.equal(result.ok, ok, `status ${status}`);
    assert.match(result.message, message);
  }
});

test("testVisionKey sends the key in a header, never the URL", async () => {
  let seenURL = "";
  let seenKey = "";
  const fakeFetch = (async (url: string, init: RequestInit) => {
    seenURL = url;
    seenKey = new Headers(init.headers).get("X-Goog-Api-Key") ?? "";
    return new Response("{}", { status: 200 });
  }) as unknown as typeof fetch;
  const result = await testVisionKey("secret-key", fakeFetch);
  assert.equal(result.ok, true);
  assert.equal(seenKey, "secret-key");
  assert.ok(!seenURL.includes("secret-key"));
});
