import { loadSettings, saveSettings } from "../shared/storage";
import type { WorkerRequest } from "../shared/types";
import { createPatternRule, isBuilderRule, uniqueRuleName, updatePattern, valuesFor, type PatternValues, type Rule, type RuleSchema, type RuleSet } from "./rule-model";
import { testVisionKey } from "./vision-key";

const visionForm = document.querySelector<HTMLFormElement>("#vision-settings")!;
const visionAPIKey = document.querySelector<HTMLInputElement>("#vision-api-key")!;
const visionKeyState = document.querySelector<HTMLElement>("#vision-key-state")!;
const visionStatus = document.querySelector<HTMLElement>("#vision-status")!;
const visionUsage = document.querySelector<HTMLElement>("#vision-usage")!;
const maxPhotosInput = document.querySelector<HTMLInputElement>("#max-photos")!;
const monthlyCapInput = document.querySelector<HTMLInputElement>("#monthly-cap")!;
const autoRun = document.querySelector<HTMLInputElement>("#auto-run")!;
const testKeyButton = document.querySelector<HTMLButtonElement>("#test-vision-key")!;
const removeKeyButton = document.querySelector<HTMLButtonElement>("#remove-vision-key")!;

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
visionForm.addEventListener("submit", (event) => {
	event.preventDefault();
	void saveVision();
});
testKeyButton.addEventListener("click", () => void checkKey());
removeKeyButton.addEventListener("click", () => void removeKey());
loadRulesButton.addEventListener("click", () => void loadRules());
reloadRulesButton.addEventListener("click", () => void loadRules());
saveRulesButton.addEventListener("click", () => void saveRules());
newRuleForm.addEventListener("submit", (event) => {
	event.preventDefault();
	addRule();
});

async function restore(): Promise<void> {
	try {
		const settings = await loadSettings();
		maxPhotosInput.value = String(settings.maxPhotos);
		monthlyCapInput.value = String(settings.monthlyCap);
		autoRun.checked = settings.autoRun;
		showKeyState(settings.visionApiKey);
		await refreshUsage(settings.monthlyCap);
	} catch (error) {
		setVisionStatus(messageFor(error), true);
	}
}

function showKeyState(savedKey: string): void {
	const saved = savedKey.trim() !== "";
	visionKeyState.textContent = saved ? "A key is saved. Paste a new key below to replace it." : "No key saved yet. Photo checks are off.";
	removeKeyButton.hidden = !saved;
}

function parseLimit(input: HTMLInputElement, min: number, max: number): number | undefined {
	const text = input.value.trim();
	if (!/^\d+$/.test(text)) return undefined;
	const value = Number(text);
	return Number.isSafeInteger(value) && value >= min && value <= max ? value : undefined;
}

async function saveVision(): Promise<void> {
	const maxPhotos = parseLimit(maxPhotosInput, 1, 24);
	if (maxPhotos === undefined) {
		setVisionStatus("Photos to check must be a whole number from 1 to 24.", true);
		return;
	}
	const monthlyCap = parseLimit(monthlyCapInput, 1, 1_000_000);
	if (monthlyCap === undefined) {
		setVisionStatus("The monthly limit must be a whole number from 1 to 1,000,000.", true);
		return;
	}
	try {
		const current = await loadSettings();
		const visionApiKey = visionAPIKey.value.trim() || current.visionApiKey;
		await saveSettings({ ...current, visionApiKey, maxPhotos, monthlyCap, autoRun: autoRun.checked });
		visionAPIKey.value = "";
		showKeyState(visionApiKey);
		setVisionStatus(visionApiKey ? "Saved. Photo checks are on." : "Saved. Photo checks stay off until you add a key.");
		await refreshUsage(monthlyCap);
	} catch (error) {
		setVisionStatus(messageFor(error), true);
	}
}

async function checkKey(): Promise<void> {
	try {
		const apiKey = visionAPIKey.value.trim() || (await loadSettings()).visionApiKey.trim();
		if (!apiKey) {
			setVisionStatus("Paste a Google Cloud Vision API key to test.", true);
			return;
		}
		setVisionStatus("Testing key…");
		const result = await testVisionKey(apiKey);
		setVisionStatus(result.message, !result.ok);
	} catch (error) {
		setVisionStatus(messageFor(error), true);
	}
}

async function removeKey(): Promise<void> {
	try {
		const current = await loadSettings();
		await saveSettings({ ...current, visionApiKey: "" });
		visionAPIKey.value = "";
		showKeyState("");
		setVisionStatus("Key removed. Photo checks are off.");
	} catch (error) {
		setVisionStatus(messageFor(error), true);
	}
}

async function refreshUsage(cap: number): Promise<void> {
	try {
		const usage = await requestWorker<{ web_detection: number; text_detection: number }>({ type: "USAGE_GET" }, "usage");
		visionUsage.textContent = `This month: ${usage.web_detection} of ${cap} reverse-image checks, ${usage.text_detection} of ${cap} watermark checks.`;
	} catch {
		visionUsage.textContent = "";
	}
}

async function loadRules(): Promise<void> {
	setRulesStatus("Loading rules…");
	try {
		const [loadedRules, schema] = await Promise.all([
			requestWorker<RuleSet>({ type: "RULES_GET" }, "rules"),
			requestWorker<RuleSchema>({ type: "RULES_SCHEMA" }, "schema")
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
		ruleSet = structuredClone(await requestWorker<RuleSet>({ type: "RULES_PUT", rules: set }, "rules"));
		renderRules();
		setRulesStatus("Rules saved. Craig checked and applied the complete rule set.");
	} catch (error) {
		setRulesStatus(messageFor(error), true);
	}
}

async function requestWorker<T>(message: WorkerRequest, field: string): Promise<T> {
	const response = await chrome.runtime.sendMessage(message) as { ok: boolean; error?: string } & Record<string, unknown>;
	if (!response?.ok) throw new Error(response?.error || "Craig's background engine did not respond.");
	return response[field] as T;
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

function setVisionStatus(message: string, error = false): void {
	visionStatus.textContent = message;
	visionStatus.style.color = error ? "#ad3935" : "#28643c";
}

function setRulesStatus(message: string, error = false): void {
	ruleStatus.textContent = message;
	ruleStatus.style.color = error ? "#ad3935" : "#28643c";
}

function messageFor(error: unknown): string {
	return error instanceof Error ? error.message : "Something went wrong. Try again.";
}
