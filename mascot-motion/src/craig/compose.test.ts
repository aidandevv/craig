import assert from "node:assert/strict";
import { test } from "node:test";
import { compose, gridToText } from "./compose.ts";
import { CANVAS_HEIGHT, CANVAS_WIDTH } from "./layer.ts";
import type { Layer } from "./layer.ts";

const layer = (overrides: Partial<Layer>): Layer => ({ name: "probe", x: 0, y: 0, rows: ["#"], ...overrides });

test("an empty pose composes to a transparent canvas", () => {
	const grid = compose([]);
	assert.equal(grid.length, CANVAS_HEIGHT);
	for (const row of grid) {
		assert.equal(row, " ".repeat(CANVAS_WIDTH));
	}
});

test("a layer lands at its offset", () => {
	const grid = compose([layer({ x: 2, y: 3, rows: ["#.", ".#"] })]);
	assert.equal(grid[3].slice(0, 5), "  #. ");
	assert.equal(grid[4].slice(0, 5), "  .# ");
});

test("upper layers cover lower ones, but transparent cells do not", () => {
	const grid = compose([layer({ name: "lower", rows: ["###"] }), layer({ name: "upper", rows: [". ."] })]);
	assert.equal(grid[0].slice(0, 3), ".#.");
});

test("rejects an empty layer", () => {
	assert.throws(() => compose([layer({ name: "nose", rows: [] })]), /Layer "nose" is empty/);
});

test("rejects uneven rows", () => {
	assert.throws(() => compose([layer({ name: "nose", rows: ["##", "#"] })]), /Layer "nose" row 1 is 1 wide; expected 2/);
});

test("rejects unknown characters", () => {
	assert.throws(() => compose([layer({ name: "nose", rows: ["#x"] })]), /Layer "nose" row 0 has unknown character "x"/);
});

test("rejects layers that leave the canvas", () => {
	assert.throws(() => compose([layer({ name: "hat", x: CANVAS_WIDTH - 1, rows: ["##"] })]), /Layer "hat" .*outside/);
	assert.throws(() => compose([layer({ name: "hat", y: -1 })]), /Layer "hat" .*outside/);
	assert.throws(() => compose([layer({ name: "hat", x: 0.5 })]), /Layer "hat" .*outside/);
});

test("gridToText writes one line per row and shows transparent cells as underscores", () => {
	assert.equal(gridToText(["#. ", " .#"]), "#._\n_.#\n");
});

test("composes onto a larger canvas when one is given, for scenes around Craig", () => {
	const grid = compose([layer({ x: 60, y: 90, rows: ["#"] })], { height: 96, width: 112 });
	assert.equal(grid.length, 96);
	assert.equal(grid[0].length, 112);
	assert.equal(grid[90][60], "#");
});

test("rejects layers that leave a given canvas", () => {
	assert.throws(
		() => compose([layer({ name: "gull", x: 110, rows: ["###"] })], { height: 96, width: 112 }),
		/Layer "gull" .*outside/,
	);
});
