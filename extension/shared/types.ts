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
  image_matches?: ImageMatchEvidence[];
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
  daemonUrl: string;
  token: string;
  autoRun: boolean;
}

export const defaultSettings: Settings = {
  daemonUrl: "http://127.0.0.1:8765",
  token: "",
  autoRun: false
};

export type WorkerRequest =
  | { type: "ANALYZE_LISTING"; listing: ListingPayload; force?: boolean }
  | { type: "OPEN_OPTIONS" };

export type WorkerResponse =
  | { ok: true; assessment: Assessment; cached: boolean; trace: TraceEvent[] }
  | { ok: false; error: string; trace: TraceEvent[] };
