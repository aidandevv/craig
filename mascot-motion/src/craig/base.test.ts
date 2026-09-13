import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { basePose } from "./base.ts";
import { compose, gridToText } from "./compose.ts";
import { CANVAS_HEIGHT, CANVAS_WIDTH } from "./layer.ts";
import { badge } from "./layers/hat.ts";

test("the base pose stacks parts in the spec's order", () => {
	assert.deepEqual(
		basePose.map((layer) => layer.name),
		["legs", "arms", "coat", "epaulettes", "head", "mouth", "mustache", "nose", "eyes", "brows", "hat", "badge"],
	);
});

test("every base layer is valid and the pose fills the 56x80 canvas", () => {
	const grid = compose(basePose);
	assert.equal(grid.length, CANVAS_HEIGHT);
	for (const row of grid) {
		assert.equal(row.length, CANVAS_WIDTH);
	}
});

test("the base pose is mirror-symmetric outside the hat badge", () => {
	const inBadge = (x: number, y: number): boolean =>
		x >= badge.x && x < badge.x + badge.rows[0].length && y >= badge.y && y < badge.y + badge.rows.length;
	const mismatches: string[] = [];
	compose(basePose).forEach((row, y) => {
		for (let x = 0; x < CANVAS_WIDTH / 2; x++) {
			const mirror = CANVAS_WIDTH - 1 - x;
			if (!inBadge(x, y) && row[x] !== row[mirror]) {
				mismatches.push(`row ${y}: col ${x} "${row[x]}" vs col ${mirror} "${row[mirror]}"`);
			}
		}
	});
	assert.deepEqual(mismatches, []);
});

test("the base pose matches the approved snapshot", () => {
	const snapshot = readFileSync(new URL("./base.snapshot.txt", import.meta.url), "utf8");
	assert.equal(gridToText(compose(basePose)), snapshot);
});
