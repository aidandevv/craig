import { loadSettings, saveSettings } from "../shared/storage";
import { createPatternRule, isBuilderRule, uniqueRuleName, updatePattern, valuesFor, type PatternValues, type Rule, type RuleSchema, type RuleSet } from "./rule-model";

const form = document.querySelector<HTMLFormElement>("#settings")!;
const daemonURL = document.querySelector<HTMLInputElement>("#daemon-url")!;
const token = document.querySelector<HTMLInputElement>("#token")!;
const autoRun = document.querySelector<HTMLInputElement>("#auto-run")!;
const settingsStatus = document.querySelector<HTMLElement>("#settings-status")!;
const testConnection = document.querySelector<HTMLButtonElement>("#test-connection")!;

const loadRulesButton = document.querySelector<HTMLButtonElement>("#load-rules")!;
const ruleEditor = document.querySelector<HTMLElement>("#rule-editor")!;
const ruleStatus = document.querySelector<HTMLElement>("#rules-status")!;
const ruleList = document.querySelector<HTMLElement>("#rule-list")!;
const newRuleForm = document.querySelector<HTMLFormElement>("#new-rule")!;
const newRuleName = document.querySelector<HTMLInputElement>("#new-rule-name")!;
const newRulePreset = document.querySelector<HTMLSelectElement>("#new-rule-preset")!;
const newRuleScope = document.querySelector<HTMLSelectElement>("#new-rule-scope")!;
const newRuleSeverity = document.querySelector<HTMLSelectElement>("#new-rule-severity")!;
const saveRulesButton = document.querySelector<HTMLButtonElement>("#save-rules")!;
const reloadRulesButton = document.querySelector<HTMLButtonElement>("#reload-rules")!;

let ruleSet: RuleSet | undefined;
let ruleSchema: RuleSchema | undefined;

void restore();
form.addEventListener("submit", (event) => {
	event.preventDefault();
	void save();
});
testConnection.addEventListener("click", () => void checkDaemon());
loadRulesButton.addEventListener("click", () => void loadRules());
reloadRulesButton.addEventListener("click", () => void loadRules());
saveRulesButton.addEventListener("click", () => void saveRules());
newRuleForm.addEventListener("submit", (event) => {
	event.preventDefault();
	addRule();
});

async function restore(): Promise<void> {
	const settings = await loadSettings();
	daemonURL.value = settings.daemonUrl;
	token.value = settings.token;
	autoRun.checked = settings.autoRun;
}

async function save(): Promise<void> {
	const normalizedURL = validateDaemonURL(daemonURL.value);
	if (!normalizedURL) {
		setSettingsStatus("Use a localhost URL such as http://127.0.0.1:8765.", true);
		return;
	}
	if (!token.value.trim()) {
		setSettingsStatus("Paste the bearer token from `craig-extension config token`.", true);
		return;
	}
	await saveSettings({ daemonUrl: normalizedURL, token: token.value.trim(), autoRun: autoRun.checked });
	daemonURL.value = normalizedURL;
	setSettingsStatus("Settings saved. Manual analysis always refreshes the current listing.");
}

async function checkDaemon(): Promise<void> {
	const normalizedURL = validateDaemonURL(daemonURL.value);
	if (!normalizedURL) {
		setSettingsStatus("Use a localhost URL before testing the daemon.", true);
		return;
	}
	setSettingsStatus("Checking local daemon…");
	try {
		const response = await fetch(`${normalizedURL}/healthz`, { signal: AbortSignal.timeout(4_000) });
		if (!response.ok) throw new Error(`HTTP ${response.status}`);
		setSettingsStatus("Local daemon is running.");
	} catch {
		setSettingsStatus("Could not reach the local daemon. Start it with `craig-extension daemon`.", true);
	}
}

async function loadRules(): Promise<void> {
	setRulesStatus("Loading rules from the daemon…");
	try {
		const [loadedRules, schema] = await Promise.all([
			requestDaemon<RuleSet>("/api/rules"),
			requestDaemon<RuleSchema>("/api/rules/schema")
		]);
		ruleSet = structuredClone(loadedRules);
		ruleSchema = schema;
		ruleEditor.hidden = false;
		populateNewRuleControls();
		renderRules();
		const editable = Object.values(ruleSet.rules).filter((rule) => isBuilderRule(rule, ruleSchema!)).length;
		setRulesStatus(`Loaded ${Object.keys(ruleSet.rules).length} rules. ${editable} can be edited visually; the rest will be preserved read-only.`);
	} catch (error) {
		setRulesStatus(messageFor(error), true);
	}
}

function populateNewRuleControls(): void {
	const schema = requireSchema();
	fillSelect(newRulePreset, presetNames(schema));
	fillSelect(newRuleScope, schema.scopes);
	fillSelect(newRuleSeverity, schema.severities);
	newRuleName.placeholder = `e.g. ${newRulePreset.value || "suspicious_rule"}`;
}

function addRule(): void {
	const set = requireRuleSet();
	const schema = requireSchema();
	const requested = newRuleName.value.trim();
	if (requested && !/^[a-z][a-z0-9_]*$/.test(requested)) {
		setRulesStatus("Rule keys must start with a lowercase letter and use only lowercase letters, numbers, and underscores.", true);
		return;
	}
	const preset = newRulePreset.value;
	if (!schema.presets[preset]) {
		setRulesStatus("Choose a known preset before adding a rule.", true);
		return;
	}
	const name = requested || uniqueRuleName(set.rules, preset);
	if (set.rules[name]) {
		setRulesStatus(`A rule named ${name} already exists. Choose a different key.`, true);
		return;
	}
	set.rules[name] = createPatternRule({ scope: newRuleScope.value, severity: newRuleSeverity.value, preset, enabled: true });
	newRuleName.value = "";
	renderRules();
	setRulesStatus(`Added ${name}. Save all rules to apply it.`);
}

function renderRules(): void {
	const set = requireRuleSet();
	const schema = requireSchema();
	ruleList.replaceChildren();
	for (const name of Object.keys(set.rules).sort()) {
		const rule = set.rules[name];
		ruleList.append(isBuilderRule(rule, schema) ? editableRuleCard(name, rule) : readonlyRuleCard(name, rule));
	}
}

function editableRuleCard(name: string, rule: Rule): HTMLElement {
	const document = ruleList.ownerDocument;
	const card = document.createElement("article");
	card.className = "rule-card";
	card.append(heading(document, name, rule.description || "Preset pattern rule"));
	const values = valuesFor(rule);
	const grid = document.createElement("div");
	grid.className = "rule-grid";
	const scope = labeledSelect(document, "Look in", requireSchema().scopes, values.scope);
	const preset = labeledSelect(document, "Suspicious pattern", presetNames(requireSchema()), values.preset);
	const severity = labeledSelect(document, "Severity", requireSchema().severities, values.severity);
	grid.append(scope.label, preset.label, severity.label);
	card.append(grid);

	const enabled = document.createElement("input");
	enabled.type = "checkbox";
	enabled.checked = values.enabled;
	const enabledLabel = document.createElement("label");
	enabledLabel.className = "check";
	enabledLabel.append(enabled, document.createTextNode("Enabled"));
	card.append(enabledLabel);

	const synchronize = () => {
		const changed: PatternValues = { scope: scope.select.value, severity: severity.select.value, preset: preset.select.value, enabled: enabled.checked };
		requireRuleSet().rules[name] = updatePattern(rule, changed);
		setRulesStatus(`Updated ${name}. Save all rules to apply changes.`);
	};
	for (const control of [scope.select, preset.select, severity.select, enabled]) control.addEventListener("change", synchronize);

	const presetInfo = requireSchema().presets[values.preset];
	const details = document.createElement("details");
	const summary = document.createElement("summary");
	summary.textContent = "View preset patterns";
	const patterns = document.createElement("pre");
	patterns.textContent = presetInfo.patterns.join("\n");
	details.append(summary, patterns);
	card.append(details);

	const remove = document.createElement("button");
	remove.type = "button";
	remove.className = "danger";
	remove.textContent = "Remove rule";
	remove.addEventListener("click", () => {
		delete requireRuleSet().rules[name];
		renderRules();
		setRulesStatus(`Removed ${name}. Save all rules to apply changes.`);
	});
	card.append(remove);
	return card;
}

function readonlyRuleCard(name: string, rule: Rule): HTMLElement {
	const document = ruleList.ownerDocument;
	const card = document.createElement("article");
	card.className = "rule-card readonly";
	card.append(heading(document, name, `${String(rule.type)} · preserved read-only`));
	const explanation = document.createElement("p");
	explanation.textContent = "This hand-written rule is outside the preset builder vocabulary. It will be sent back unchanged when you save other rule cards.";
	const details = document.createElement("details");
	const summary = document.createElement("summary");
	summary.textContent = "View rule data";
	const source = document.createElement("pre");
	source.textContent = JSON.stringify(rule, null, 2);
	details.append(summary, source);
	card.append(explanation, details);
	return card;
}

async function saveRules(): Promise<void> {
	const set = requireRuleSet();
	setRulesStatus("Saving rules…");
	try {
		await requestDaemon("/api/rules", { method: "PUT", body: JSON.stringify(set) });
		setRulesStatus("Rules saved. The daemon compiled the complete rule set successfully.");
	} catch (error) {
		setRulesStatus(messageFor(error), true);
	}
}

async function requestDaemon<T>(path: string, init: RequestInit = {}): Promise<T> {
	const settings = await loadSettings();
	const baseURL = validateDaemonURL(settings.daemonUrl);
	if (!baseURL || !settings.token.trim()) {
		throw new Error("Save a localhost daemon URL and bearer token before loading rules.");
	}
	const headers = new Headers(init.headers);
	headers.set("Authorization", `Bearer ${settings.token}`);
	if (init.body) headers.set("Content-Type", "application/json");
	const response = await fetch(`${baseURL}${path}`, { ...init, headers, signal: AbortSignal.timeout(8_000) });
	if (!response.ok) {
		const payload = await response.json().catch(() => ({})) as { error?: string };
		throw new Error(payload.error || `Daemon returned HTTP ${response.status}.`);
	}
	return await response.json() as T;
}

function heading(document: Document, name: string, description: string): HTMLElement {
	const heading = document.createElement("h3");
	heading.textContent = name;
	const summary = document.createElement("p");
	summary.textContent = description;
	const wrapper = document.createElement("div");
	wrapper.append(heading, summary);
	return wrapper;
}

function labeledSelect(document: Document, label: string, values: string[], selected: string): { label: HTMLLabelElement; select: HTMLSelectElement } {
	const field = document.createElement("label");
	field.textContent = label;
	const select = document.createElement("select");
	fillSelect(select, values, selected);
	field.append(select);
	return { label: field, select };
}

function fillSelect(select: HTMLSelectElement, values: string[], selected?: string): void {
	select.replaceChildren();
	for (const value of values) {
		const option = document.createElement("option");
		option.value = value;
		option.textContent = value.replace(/_/g, " ");
		option.selected = value === (selected || values[0]);
		select.append(option);
	}
}

function presetNames(schema: RuleSchema): string[] {
	return Object.keys(schema.presets).sort();
}

function requireRuleSet(): RuleSet {
	if (!ruleSet) throw new Error("Load rules before editing them.");
	return ruleSet;
}

function requireSchema(): RuleSchema {
	if (!ruleSchema) throw new Error("Load the rule schema before editing rules.");
	return ruleSchema;
}

function validateDaemonURL(value: string): string | undefined {
	try {
		const parsed = new URL(value.trim());
		if (parsed.protocol !== "http:" || (parsed.hostname !== "127.0.0.1" && parsed.hostname !== "localhost")) return undefined;
		return parsed.toString().replace(/\/$/, "");
	} catch {
		return undefined;
	}
}

function setSettingsStatus(message: string, error = false): void {
	settingsStatus.textContent = message;
	settingsStatus.style.color = error ? "#ad3935" : "#28643c";
}

function setRulesStatus(message: string, error = false): void {
	ruleStatus.textContent = message;
	ruleStatus.style.color = error ? "#ad3935" : "#28643c";
}

function messageFor(error: unknown): string {
	return error instanceof Error ? error.message : "The daemon request failed.";
}
