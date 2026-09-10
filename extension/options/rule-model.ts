export interface RuleMatch {
	preset?: string;
	custom?: string[];
	regex?: boolean;
}

export interface Rule {
	type: string;
	severity: string;
	description?: string;
	enabled?: boolean;
	weight?: number;
	hard?: boolean;
	scope?: string;
	match?: RuleMatch;
	[key: string]: unknown;
}

export interface RuleSet {
	version: string;
	severities: Record<string, number>;
	risk_bands: Record<string, number>;
	rules: Record<string, Rule>;
	[key: string]: unknown;
}

export interface Preset {
	description: string;
	patterns: string[];
}

export interface RuleSchema {
	scopes: string[];
	severities: string[];
	presets: Record<string, Preset>;
}

export interface PatternValues {
	scope: string;
	severity: string;
	preset: string;
	enabled: boolean;
}

// A visual card handles only the intentionally finite, preset-backed form of
// pattern_match. Any hand-written expression remains visible but read-only so
// saving a builder edit can round-trip it without approximation or deletion.
export function isBuilderRule(rule: Rule, schema: RuleSchema): boolean {
	return rule.type === "pattern_match" &&
		typeof rule.scope === "string" && schema.scopes.includes(rule.scope) &&
		schema.severities.includes(rule.severity) &&
		typeof rule.match?.preset === "string" && schema.presets[rule.match.preset] !== undefined &&
		rule.match.custom === undefined;
}

export function valuesFor(rule: Rule): PatternValues {
	return {
		scope: String(rule.scope),
		severity: rule.severity,
		preset: String(rule.match?.preset),
		enabled: rule.enabled !== false
	};
}

// updatePattern preserves every non-builder field (description, hard, weight,
// and future schema fields) while changing only the explicit visual controls.
export function updatePattern(rule: Rule, values: PatternValues): Rule {
	return {
		...rule,
		scope: values.scope,
		severity: values.severity,
		enabled: values.enabled,
		match: { preset: values.preset }
	};
}

export function createPatternRule(values: PatternValues): Rule {
	return {
		type: "pattern_match",
		scope: values.scope,
		severity: values.severity,
		enabled: values.enabled,
		match: { preset: values.preset }
	};
}

export function uniqueRuleName(rules: Record<string, Rule>, preferred: string): string {
	const root = preferred.trim().toLowerCase().replace(/[^a-z0-9_]+/g, "_").replace(/^_+|_+$/g, "") || "new_rule";
	if (rules[root] === undefined) return root;
	for (let number = 2; ; number++) {
		const candidate = `${root}_${number}`;
		if (rules[candidate] === undefined) return candidate;
	}
}
