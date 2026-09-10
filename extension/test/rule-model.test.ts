import assert from "node:assert/strict";
import test from "node:test";

import { createPatternRule, isBuilderRule, uniqueRuleName, updatePattern, type Rule, type RuleSchema } from "../options/rule-model";

const schema: RuleSchema = {
	scopes: ["title", "description", "whole_post"],
	severities: ["red", "risk", "green"],
	presets: { urgency_pressure: { description: "Urgency", patterns: ["must go"] } }
};

test("builder recognizes only its finite preset-backed rule vocabulary", () => {
	const editable: Rule = { type: "pattern_match", scope: "whole_post", severity: "risk", match: { preset: "urgency_pressure" } };
	assert.equal(isBuilderRule(editable, schema), true);
	assert.equal(isBuilderRule({ ...editable, match: { custom: ["hand written"], regex: true } }, schema), false);
	assert.equal(isBuilderRule({ type: "image_analysis", severity: "red" }, schema), false);
});

test("builder updates retain hand-authored fields and generate collision-free names", () => {
	const existing: Rule = {
		type: "pattern_match", scope: "whole_post", severity: "risk", hard: true,
		description: "Keep this explanation", match: { preset: "urgency_pressure" }
	};
	const changed = updatePattern(existing, { scope: "title", severity: "red", preset: "urgency_pressure", enabled: false });
	assert.equal(changed.hard, true);
	assert.equal(changed.description, "Keep this explanation");
	assert.deepEqual(changed.match, { preset: "urgency_pressure" });
	assert.equal(uniqueRuleName({ urgency_pressure: existing }, "urgency pressure"), "urgency_pressure_2");
	assert.deepEqual(createPatternRule({ scope: "title", severity: "risk", preset: "urgency_pressure", enabled: true }).match, { preset: "urgency_pressure" });
});
