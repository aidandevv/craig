import assert from "node:assert/strict";
import { test } from "node:test";
import { basePose } from "./base.ts";
import { compose } from "./compose.ts";
import { CANVAS_WIDTH } from "./layer.ts";
import type { Layer, Pose } from "./layer.ts";
import { mirrorLayer, mirrorPose } from "./mirror.ts";

test("mirrorLayer flips a layer's pixels and mirrors its position", () => {
	const layer: Layer = { name: "probe", x: 1, y: 2, rows: ["#. ", "  #"] };
	assert.deepEqual(mirrorLayer(layer), { name: "probe", x: CANVAS_WIDTH - 4, y: 2, rows: [" .#", "#  "] });
});

test("a layer that keeps its orientation moves without flipping", () => {
	const layer: Layer = { keepOrientation: true, name: "badge", x: 0, y: 0, rows: ["#."] };
	assert.deepEqual(mirrorLayer(layer), { keepOrientation: true, name: "badge", x: CANVAS_WIDTH - 2, y: 0, rows: ["#."] });
});

test("mirroring a pose twice restores it", () => {
	const pose: Pose = [{ name: "probe", x: 3, y: 4, rows: ["##.", "#  "] }];
	assert.deepEqual(mirrorPose(mirrorPose(pose)), pose);
});

test("the symmetric base pose is its own mirror image, C and all", () => {
	assert.deepEqual(compose(mirrorPose(basePose)), compose(basePose));
});
