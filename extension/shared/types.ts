export interface Contact {
  email?: string;
  phone?: string;
  relay_only?: boolean;
  app_only?: boolean;
}

export interface ListingPayload {
  marketplace: string;
  listing_url: string;
  title: string;
  price?: number;
  currency?: string;
	rent_period?: string;
	bedrooms?: number;
	zip_code?: string;
  description?: string;
  images?: string[];
  captions?: string[];
  contact: Contact;
  posted_at?: string;
}

export interface Finding {
  rule: string;
  label: string;
  detail?: string;
  weight?: number;
	text_match?: TextMatchEvidence;
  image_matches?: ImageMatchEvidence[];
}

export interface TextMatchEvidence {
	before?: string;
	match: string;
	after?: string;
}

export interface ImageMatchEvidence {
  listing_image_url: string;
  source_page_url: string;
  source_image_url?: string;
}

export interface NotEvaluated {
  rule: string;
  reason: string;
}

export interface Assessment {
  image_coverage?: { signal: string; checked: number; total: number; checked_at: string }[];
  image_candidates?: ImageMatchEvidence[];
  risk_score: number;
  risk_band: "low" | "caution" | "elevated" | "high" | string;
  hard_flagged: boolean;
  coverage: { ran: number; enabled: number };
  high_risk: Finding[];
  potentially_risky: Finding[];
  positive_signals: Finding[];
  passed_checks: string[];
  not_evaluated: NotEvaluated[];
	analysis_time_ms: number;
}

// Trace events are emitted by the extension and local daemon while processing
// a single analysis. They never contain the bearer token, Vision API key,
// listing text, or image URLs.
export interface TraceEvent {
	timestamp: string;
	step: string;
	message: string;
}

export interface Settings {
  visionApiKey: string;
  monthlyCap: number;
  maxPhotos: number;
  daemonUrl: string;
  daemonId: string;
  token: string;
  autoRun: boolean;
}

export const defaultSettings: Settings = {
  visionApiKey: "",
  monthlyCap: 999,
  maxPhotos: 4,
  daemonUrl: "http://127.0.0.1:8765",
  daemonId: "",
  token: "",
  autoRun: false
};

export type WorkerRequest =
  | { type: "ANALYZE_LISTING"; listing: ListingPayload; force?: boolean }
  | { type: "OPEN_OPTIONS" }
  | { type: "RULES_GET" }
  | { type: "RULES_PUT"; rules: unknown }
  | { type: "RULES_SCHEMA" }
  | { type: "USAGE_GET" };

export type WorkerResponse =
  | { ok: true; assessment: Assessment; cached: boolean; trace: TraceEvent[] }
  | { ok: false; error: string; trace: TraceEvent[] };
