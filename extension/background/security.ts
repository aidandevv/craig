import type { ListingPayload } from "../shared/types";

export function craigslistURL(value: unknown): string | undefined {
  if (typeof value !== "string" || value.length > 4096) return undefined;
  try {
    const url = new URL(value);
    if (url.protocol !== "https:" || url.username || url.password || url.port ||
        !url.hostname.endsWith(".craigslist.org")) return undefined;
    url.hash = "";
    return url.href;
  } catch { return undefined; }
}

export function contentSender(sender: chrome.runtime.MessageSender): boolean {
  return sender.id === chrome.runtime.id && sender.frameId === 0 &&
    sender.tab?.id !== undefined && !!craigslistURL(sender.url);
}

export function optionsSender(sender: chrome.runtime.MessageSender): boolean {
  return sender.id === chrome.runtime.id && sender.url === chrome.runtime.getURL("dist/options/index.html");
}

// Bound the structured message before JSON.stringify allocates another copy.
export function boundedPayload(value: unknown): boolean {
  let nodes = 0, bytes = 0;
  const visit = (item: unknown, depth: number): boolean => {
    if (++nodes > 20_000 || depth > 24) return false;
    if (typeof item === "string") bytes += item.length * 3;
    else if (item && typeof item === "object") {
      for (const key in item) {
        if (!Object.prototype.hasOwnProperty.call(item, key)) continue;
        const child = (item as Record<string, unknown>)[key];
        bytes += key.length * 3;
        if (bytes > 512_000 || !visit(child, depth + 1)) return false;
      }
    }
    return bytes <= 512_000;
  };
  return visit(value, 0);
}

export function listingFromSender(value: unknown, sender: chrome.runtime.MessageSender): value is ListingPayload {
  if (!value || typeof value !== "object" || !boundedPayload(value)) return false;
  const listing = value as ListingPayload;
  return listing.marketplace === "craigslist" && typeof listing.title === "string" &&
    !!craigslistURL(listing.listing_url) && craigslistURL(listing.listing_url) === craigslistURL(sender.url) &&
    (listing.images === undefined || Array.isArray(listing.images) && listing.images.length <= 24 && listing.images.every(v => typeof v === "string" && v.length <= 4096)) &&
    (listing.captions === undefined || Array.isArray(listing.captions) && listing.captions.length <= 128 && listing.captions.every(v => typeof v === "string" && v.length <= 4096));
}
