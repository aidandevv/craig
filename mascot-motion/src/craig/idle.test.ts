import assert from "node:assert/strict";
import { test } from "node:test";
import { basePose } from "./base.ts";
import { compose } from "./compose.ts";
import { idleDuration, idleFps, idlePoseAt, idlePoses, shiftPose } from "./idle.ts";
import { CANVAS_HEIGHT, CANVAS_WIDTH } from "./layer.ts";

test("shiftPose moves every layer by whole pixels", () => {
	const shifted = shiftPose(basePose, 0, 1);
	assert.deepEqual(
		shifted.map((layer) => layer.y),
		basePose.map((layer) => layer.y + 1),
	);
	assert.deepEqual(
		shifted.map((layer) => layer.x),
		basePose.map((layer) => layer.x),
	);
});

test("the idle loop starts and ends on the base pose", () => {
	assert.deepEqual(compose(idlePoseAt(0)), compose(basePose));
	assert.deepEqual(compose(idlePoseAt(idleDuration - 1)), compose(basePose));
});

test("the loop holds one drawing per cell, at a stop-motion rate", () => {
	assert.equal(idlePoses.length, idleDuration);
	assert.ok(idleFps >= 6 && idleFps <= 12, `idleFps ${idleFps} is a stop-motion rate`);
});

test("every idle frame composes to a full canvas", () => {
	for (let frame = 0; frame < idleDuration; frame++) {
		const grid = compose(idlePoseAt(frame));
		assert.equal(grid.length, CANVAS_HEIGHT, `frame ${frame}`);
		assert.equal(grid[0].length, CANVAS_WIDTH, `frame ${frame}`);
	}
});

test("he blinks at some point in the loop", () => {
	const closed = idlePoses.filter((pose) => compose(pose)[22].slice(19, 23) === "####");
	assert.ok(closed.length >= 2, `${closed.length} frames show closed eyes`);
});

test("he breathes: some frames sit one pixel lower", () => {
	const bobbed = idlePoses.filter((pose) => compose(pose)[0].indexOf("#") === -1);
	assert.ok(bobbed.length >= 4, `${bobbed.length} frames are bobbed down`);
});
