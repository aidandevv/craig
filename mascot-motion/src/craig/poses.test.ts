import assert from "node:assert/strict";
import { test } from "node:test";
import { compose } from "./compose.ts";
import { CANVAS_HEIGHT, CANVAS_WIDTH } from "./layer.ts";
import { mirrorPose } from "./mirror.ts";
import {
	cautionPose,
	elevatedConcernPose,
	hardFlagPose,
	highConcernPose,
	incompletePose,
	lowConcernPose,
	shadeEyesPose,
	sprites,
	telescopeCanePose,
	telescopeEyePose,
	telescopeSidePose,
} from "./poses.ts";

test("every sprite has a unique name", () => {
	const names = sprites.map(([name]) => name);
	assert.equal(new Set(names).size, names.length);
	assert.deepEqual(names.slice(0, 2), ["base", "telescope-side-right"]);
});

test("every sprite composes to a full 56x80 canvas", () => {
	for (const [name, pose] of sprites) {
		const grid = compose(pose);
		assert.equal(grid.length, CANVAS_HEIGHT, name);
		for (const row of grid) {
			assert.equal(row.length, CANVAS_WIDTH, name);
		}
	}
});

test("each left-facing sprite is the mirror image of its right-facing twin", () => {
	for (const [name, pose] of sprites) {
		if (!name.endsWith("-left")) {
			continue;
		}
		const twin = sprites.find(([other]) => other === `${name.slice(0, -"-left".length)}-right`);
		assert.ok(twin, `${name} has a right-facing twin`);
		assert.deepEqual(compose(pose), compose(mirrorPose(twin[1])), name);
	}
});

test("every state keeps a hand at work and differs from the base", () => {
	const base = compose(sprites[0][1]);
	for (const pose of [telescopeSidePose, telescopeCanePose, telescopeEyePose, shadeEyesPose]) {
		assert.notDeepEqual(compose(pose), base);
	}
});

test("every front-facing concern state composes and is distinct from every other one", () => {
	const concernPoses = [lowConcernPose, cautionPose, elevatedConcernPose, highConcernPose, hardFlagPose, incompletePose];
	const grids = concernPoses.map((pose) => compose(pose));
	for (let i = 0; i < grids.length; i++) {
		for (let j = i + 1; j < grids.length; j++) {
			assert.notDeepEqual(grids[i], grids[j], `pose ${i} vs pose ${j}`);
		}
	}
});

test("low concern has no gesture of its own: it's the same at-ease pose as the base", () => {
	assert.deepEqual(compose(lowConcernPose), compose(sprites[0][1]));
});

test("elevated concern and high concern each differ from the base", () => {
	const base = compose(sprites[0][1]);
	assert.notDeepEqual(compose(elevatedConcernPose), base);
	assert.notDeepEqual(compose(highConcernPose), base);
});

test("high concern and hard flag differ only by the exclamation mark", () => {
	const high = compose(highConcernPose);
	const hardFlag = compose(hardFlagPose);
	assert.notDeepEqual(high, hardFlag);
});

test("caution and incomplete differ only by the question mark", () => {
	const caution = compose(cautionPose);
	const incomplete = compose(incompletePose);
	assert.notDeepEqual(caution, incomplete);
});
