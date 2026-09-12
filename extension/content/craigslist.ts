import type { ListingPayload } from "../shared/types";

const phonePattern = /(?:\+?1[\s.-]?)?(?:\(?\d{3}\)?[\s.-]?)\d{3}[\s.-]?\d{4}/;
const emailPattern = /\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b/i;

// extractListing deliberately understands only the stable Craigslist listing
// shape. It is pure DOM-to-data code so selectors can be tested with a saved
// fixture and changed without touching the daemon's marketplace-neutral API.
export function extractListing(document: Document): ListingPayload {
  const title = textOf(document, "#titletextonly, #postingtitle .postingtitletext") ||
    document.title.replace(/\s*-\s*craigslist(?:\s|$).*/i, "").trim();
  if (!title) {
    throw new Error("This page does not look like a Craigslist listing: title not found.");
  }

  const description = postingBody(document);
  const images = imageURLs(document);
  const captions = imageCaptions(document);
  const phone = description.match(phonePattern)?.[0];
  const email = description.match(emailPattern)?.[0];
  const replyAvailable = Boolean(document.querySelector("#replylink, .reply-button, button.reply-button"));
	// A Craigslist Reply button is present on normal posts too. It is not
	// evidence that messaging is the seller's *only* route when the post exposes
	// a "show contact info" control for calling or texting. The number itself is
	// intentionally revealed by Craigslist only after a click, so it cannot be
	// found with phonePattern at extraction time.
	const directContactAvailable = Boolean(phone || email || document.querySelector(
		".show-contact, a[href^='tel:'], a[href^='mailto:']"
	));
  const postedAt = timestampOf(document);
	const attributes = attrGroupText(document);

  return compact({
    marketplace: "craigslist",
    listing_url: document.location.href,
    title,
    price: priceOf(document),
    currency: "USD",
		rent_period: rentPeriodOf(attributes),
		bedrooms: bedroomsOf(textOf(document, ".attrgroup .shared-line-bubble") || title),
		zip_code: zipCodeOf(`${title}\n${description}`),
    description,
    images,
    captions,
    contact: compact({
      email,
      phone,
		// Mark this only when there is truly no observed direct-contact route.
		relay_only: replyAvailable && !directContactAvailable
    }),
    posted_at: postedAt
  }) as ListingPayload;
}

function textOf(document: Document, selector: string): string {
  return document.querySelector(selector)?.textContent?.trim() || "";
}

function postingBody(document: Document): string {
  const body = document.querySelector("#postingbody");
  const bodyText = body ? cleanedText(body) : "";
  return [bodyText, attrGroupText(document)].filter(Boolean).join("\n").trim();
}

function cleanedText(node: Element): string {
	const copy = node.cloneNode(true) as HTMLElement;
	copy.querySelectorAll(".print-qrcode, .notices, script, style").forEach((n) => n.remove());
	return (copy.textContent || "")
		.replace(/QR Code Link to This Post/gi, "")
		.replace(/\n\s*\n+/g, "\n")
		.trim();
}

// Craigslist housing listings render structured fields — application fee
// details, broker fee details, listed by, rent period, and similar — as
// key/value spans inside .attrgroup, separate from #postingbody. Without
// this, that text never reaches any rule and fee-related scam signals in it
// go undetected rather than merely unmatched.
function attrGroupText(document: Document): string {
	const lines: string[] = [];
	document.querySelectorAll(".attrgroup > span").forEach((span) => {
		const text = span.textContent?.trim();
		if (text) lines.push(text);
	});
	return lines.join("\n");
}

function priceOf(document: Document): number | undefined {
  const price = textOf(document, "#postingtitle .price, .price");
  const digits = price.replace(/[^0-9]/g, "");
  return digits ? Number.parseInt(digits, 10) : undefined;
}

function rentPeriodOf(attributes: string): string | undefined {
	return attributes.match(/\brent\s+period\s*:\s*([a-z]+)/i)?.[1]?.toLowerCase();
}

function bedroomsOf(text: string): number | undefined {
	const bedrooms = text.match(/\b([0-4])\s*(?:br|bed(?:room)?s?)\b/i)?.[1];
	if (bedrooms !== undefined) return Number.parseInt(bedrooms, 10);
	return /\bstudio\b/i.test(text) ? 0 : undefined;
}

function zipCodeOf(text: string): string | undefined {
	return text.match(/\b(\d{5})(?:-\d{4})?\b/)?.[1];
}

function timestampOf(document: Document): string | undefined {
	const raw = document.querySelector("time.date[datetime], time[datetime]")?.getAttribute("datetime");
	if (!raw) return undefined;
	const parsed = new Date(raw);
	return Number.isNaN(parsed.valueOf()) ? undefined : parsed.toISOString();
}

function imageURLs(document: Document): string[] {
  const urls = new Set<string>();
  const add = (raw: string) => {
    if (!/^https?:/i.test(raw)) return;
    const url = new URL(raw);
    if (url.hostname === "images.craigslist.org") {
      url.pathname = url.pathname.replace(/_\d+x\d+\.(jpg|jpeg|png)$/i, "_1200x900.$1");
    }
    urls.add(url.href);
  };
  // Some gallery states expose only a subset as links. Collect each source,
  // then deduplicate after size normalization so one photo costs one check.
  document.querySelectorAll<HTMLAnchorElement>("#thumbs a[href]").forEach((node) => add(node.href));
  document.querySelectorAll<HTMLImageElement>("#thumbs img[src], .gallery img[src], #ci img[src]").forEach((node) => add(node.src));
  return [...urls];
}

function imageCaptions(document: Document): string[] {
  const captions = new Set<string>();
	document.querySelectorAll<HTMLAnchorElement>("#thumbs a[title]").forEach((node) => {
		const value = node.title;
		if (value.trim()) {
			captions.add(value.trim());
		}
	});
	document.querySelectorAll<HTMLImageElement>("#thumbs img[alt]").forEach((node) => {
		const value = node.alt;
    if (value.trim()) {
      captions.add(value.trim());
    }
  });
  return [...captions];
}

function compact<T extends Record<string, unknown>>(value: T): T {
  return Object.fromEntries(Object.entries(value).filter(([, entry]) =>
    entry !== undefined && entry !== "" && entry !== false && (!Array.isArray(entry) || entry.length > 0)
  )) as T;
}
