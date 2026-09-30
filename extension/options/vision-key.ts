const ENDPOINT = "https://vision.googleapis.com/v1/images:annotate";

export interface KeyCheck {
  ok: boolean;
  message: string;
}

// An empty batch exercises key validity and API enablement without annotating
// an image. Whether it also surfaces BILLING_DISABLED is verified manually.
export async function testVisionKey(apiKey: string, fetchImpl: typeof fetch = fetch): Promise<KeyCheck> {
  try {
    const response = await fetchImpl(ENDPOINT, {
      method: "POST",
      redirect: "error",
      headers: { "Content-Type": "application/json", "X-Goog-Api-Key": apiKey },
      body: JSON.stringify({ requests: [] }),
      signal: AbortSignal.timeout(8_000)
    });
    return classifyKeyResponse(response.status, await response.json().catch(() => ({})));
  } catch {
    return { ok: false, message: "Craig couldn't reach Google. Check your connection and try again." };
  }
}

export function classifyKeyResponse(status: number, body: unknown): KeyCheck {
  if (status >= 200 && status < 300) {
    return { ok: true, message: "Key works. Photo checks are on." };
  }
  const reasons = errorReasons(body);
  if (reasons.includes("API_KEY_INVALID")) {
    return { ok: false, message: "Google doesn't recognize this key. Copy it again from the Credentials page." };
  }
  if (reasons.includes("SERVICE_DISABLED")) {
    return { ok: false, message: "Turn on the Cloud Vision API for this key's project, wait a minute, then test again." };
  }
  if (reasons.includes("BILLING_DISABLED")) {
    return { ok: false, message: "Turn on billing for this project. The first 1,000 checks of each type every month are free." };
  }
  if (reasons.includes("API_KEY_SERVICE_BLOCKED")) {
    return { ok: false, message: "This key is restricted to other APIs. Add Cloud Vision API to its allowed APIs." };
  }
  if (status === 429) {
    return { ok: false, message: "Google is rate-limiting this key. Try again in a minute." };
  }
  return { ok: false, message: `Google returned HTTP ${status}. Check the key and try again.` };
}

function errorReasons(body: unknown): string[] {
  const details = (body as { error?: { details?: unknown } })?.error?.details;
  if (!Array.isArray(details)) return [];
  return details
    .map((detail) => (detail as { reason?: unknown })?.reason)
    .filter((reason): reason is string => typeof reason === "string");
}
