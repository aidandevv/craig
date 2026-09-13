import assert from "node:assert/strict";
import { test } from "node:test";
import { basePose } from "../../base.ts";
import { compose } from "../../compose.ts";
import { CANVAS_HEIGHT, CANVAS_WIDTH } from "../../layer.ts";
import type { Pose } from "../../layer.ts";
import { coat } from "../../layers/coat.ts";
import { badge, hat } from "../../layers/hat.ts";
import { head } from "../../layers/head.ts";
import { legs } from "../../layers/legs.ts";
import { mouth } from "../../layers/mouth.ts";
import { mustache } from "../../layers/mustache.ts";
import { nose } from "../../layers/nose.ts";
import { shadeEyesPose } from "../../poses.ts";
import { lookoutDuration, lookoutFps, lookoutFrame, lookoutFrames } from "./frames.ts";

/** Every run of one held drawing, in order. */
const runs = (): readonly (readonly [Pose, number])[] => {
	const out: [Pose, number][] = [];
	for (const pose of lookoutFrames) {
		const last = out.length === 0 ? null : out[out.length - 1];
		if (last && last[0] === pose) {
			last[1]++;
		} else {
			out.push([pose, 1]);
		}
	}
	return out;
};

test("the loop runs five seconds at 24 fps", () => {
	assert.equal(lookoutFps, 24);
	assert.equal(lookoutDuration, 120);
	assert.equal(lookoutFrames.length, lookoutDuration);
});

test("the first and last frames are the base pose, so the loop closes", () => {
	const base = compose(basePose);
	assert.deepEqual(compose(lookoutFrames[0]), base);
	assert.deepEqual(compose(lookoutFrames[lookoutDuration - 1]), base);
});

test("every frame composes to a full 56x80 canvas", () => {
	lookoutFrames.forEach((pose, frame) => {
		const grid = compose(pose);
		assert.equal(grid.length, CANVAS_HEIGHT, `frame ${frame}`);
		for (const row of grid) {
			assert.equal(row.length, CANVAS_WIDTH, `frame ${frame}`);
		}
	});
});

test("the arm lands on the lookout pose at the top of its climb", () => {
	assert.deepEqual(compose(lookoutFrames[30]), compose(shadeEyesPose));
});

test("each drawing is held, and every change is a new drawing", () => {
	const held = runs();
	assert.ok(held.length > 30, `${held.length} drawings`);
	held.forEach(([pose, length], index) => {
		assert.ok(length >= 2, `drawing ${index} is held ${length} frames`);
		if (pose !== basePose) {
			assert.ok(length <= 4, `drawing ${index} is held ${length} frames`);
		}
		if (index > 0) {
			assert.notDeepEqual(compose(pose), compose(held[index - 1][0]), `drawing ${index} repeats the one before`);
		}
	});
});

test("his hat, head, coat and legs hold still in every frame", () => {
	for (const pose of lookoutFrames) {
		for (const layer of [legs, coat, head, mouth, mustache, nose, hat, badge]) {
			assert.ok(pose.indexOf(layer) !== -1, `${layer.name} is missing`);
		}
	}
});

test("frames outside the plan clamp to its ends", () => {
	assert.equal(lookoutFrame(-4), lookoutFrames[0]);
	assert.equal(lookoutFrame(0), lookoutFrames[0]);
	assert.equal(lookoutFrame(lookoutDuration + 9), lookoutFrames[lookoutDuration - 1]);
});
