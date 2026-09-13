import assert from "node:assert/strict";
import { test } from "node:test";
import { toRects } from "./rects.ts";

test("merges runs of the same paint and skips transparent cells", () => {
	assert.deepEqual(toRects(["##. #"]), [
		{ paint: "#", width: 2, x: 0, y: 0 },
		{ paint: ".", width: 1, x: 2, y: 0 },
		{ paint: "#", width: 1, x: 4, y: 0 },
	]);
});

test("never merges across rows", () => {
	assert.deepEqual(toRects(["#", "#"]), [
		{ paint: "#", width: 1, x: 0, y: 0 },
		{ paint: "#", width: 1, x: 0, y: 1 },
	]);
});

test("a transparent grid has no rectangles", () => {
	assert.deepEqual(toRects(["   ", "   "]), []);
});
