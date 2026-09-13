import assert from "node:assert/strict";
import { test } from "node:test";
import { basePose } from "../../base.ts";
import { compose } from "../../compose.ts";
import { CANVAS_HEIGHT, CANVAS_WIDTH } from "../../layer.ts";
import type { Pose } from "../../layer.ts";
import { mirrorPose } from "../../mirror.ts";
import { telescopeEyePose, telescopeSidePose } from "../../poses.ts";
import { peekDuration, peekFps, peekFrame, peekFrames } from "./frames.ts";

/** Every run of one held drawing, in order. */
const runs = (): readonly (readonly [Pose, number])[] => {
	const out: [Pose, number][] = [];
	for (const pose of peekFrames) {
		const last = out.length === 0 ? null : out[out.length - 1];
		if (last && last[0] === pose) {
			last[1]++;
		} else {
			out.push([pose, 1]);
		}
	}
	return out;
};

/** The frames where a composed pose first matches `target`. */
const firstMatch = (target: Pose): number => {
	const wanted = compose(target);
	for (let frame = 0; frame < peekFrames.length; frame++) {
		const grid = compose(peekFrames[frame]);
		if (grid.join("\n") === wanted.join("\n")) {
			return frame;
		}
	}
	return -1;
};

test("the loop runs seven and a half seconds at 24 fps", () => {
	assert.equal(peekFps, 24);
	assert.equal(peekDuration, 180);
	assert.equal(peekFrames.length, peekDuration);
});

test("the first and last frames are the base pose, so the loop closes", () => {
	const base = compose(basePose);
	assert.deepEqual(compose(peekFrames[0]), base);
	assert.deepEqual(compose(peekFrames[peekDuration - 1]), base);
});

test("every frame composes to a full 56x80 canvas", () => {
	peekFrames.forEach((pose, frame) => {
		const grid = compose(pose);
		assert.equal(grid.length, CANVAS_HEIGHT, `frame ${frame}`);
		for (const row of grid) {
			assert.equal(row.length, CANVAS_WIDTH, `frame ${frame}`);
		}
	});
});

test("it passes through the shared spyglass poses on the way up", () => {
	const side = firstMatch(telescopeSidePose);
	const eye = firstMatch(telescopeEyePose);
	assert.ok(side > 0, "settles on the telescope-side pose");
	assert.ok(eye > side, "reaches the telescope-eye pose after it");
});

test("it peeks the other way with the mirrored eye pose", () => {
	const left = firstMatch(mirrorPose(telescopeEyePose));
	assert.ok(left > 0, "reaches the mirrored telescope-eye pose");
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

test("his hat and his boots hold still in every frame", () => {
	const base = compose(basePose);
	for (const pose of peekFrames) {
		const grid = compose(pose);
		for (let y = 0; y <= 11; y++) {
			assert.equal(grid[y], base[y], `row ${y} of the hat moved`);
		}
		for (let y = 72; y < CANVAS_HEIGHT; y++) {
			assert.equal(grid[y], base[y], `row ${y} of the boots moved`);
		}
	}
});

test("frames outside the plan clamp to its ends", () => {
	assert.equal(peekFrame(-4), peekFrames[0]);
	assert.equal(peekFrame(peekDuration + 9), peekFrames[peekDuration - 1]);
});
