import assert from "node:assert/strict";
import { test } from "node:test";
import { basePose } from "../../base.ts";
import { compose } from "../../compose.ts";
import type { Pose } from "../../layer.ts";
import { armBehindLeft, arms, armSideRight, sleeveTelescopeRight } from "../../layers/arms.ts";
import { telescopeEyePose, telescopeSidePose } from "../../poses.ts";
import {
	armBehindRight,
	FIST_HOME,
	fistDown,
	forearmRight,
	shoulderRight,
	SLEEVE_FULL,
	SLEEVE_HOME,
	sleeveDown,
	sleeveRaised,
} from "./armStates.ts";
import { craigEye } from "./lens.ts";
import { poseAtFrame, TELESCOPE_DURATION, TELESCOPE_FPS, telescopeFrames } from "./plan.ts";
import { arrivalStages, foreshortenStages, growStages, lensPose, raiseStages, reachStages } from "./poses.ts";

const flat = (pose: Pose): string => compose(pose).join("\n");
const inkCount = (pose: Pose): number => flat(pose).split("#").length - 1;

test("the loop runs between eight and ten seconds", () => {
	assert.equal(TELESCOPE_FPS, 24);
	assert.equal(TELESCOPE_DURATION, telescopeFrames.length);
	assert.ok(TELESCOPE_DURATION >= 8 * TELESCOPE_FPS, `${TELESCOPE_DURATION} frames is under eight seconds`);
	assert.ok(TELESCOPE_DURATION <= 10 * TELESCOPE_FPS, `${TELESCOPE_DURATION} frames is over ten seconds`);
});

test("the first and last frames are both the base pose, so the loop closes", () => {
	const base = flat(basePose);
	assert.equal(flat(telescopeFrames[0]), base);
	assert.equal(flat(telescopeFrames[TELESCOPE_DURATION - 1]), base);
});

test("every frame composes", () => {
	telescopeFrames.forEach((pose, frame) => {
		assert.doesNotThrow(() => compose(pose), `frame ${frame} does not compose`);
	});
});

test("poseAtFrame clamps to the ends of the loop", () => {
	assert.equal(poseAtFrame(-5), telescopeFrames[0]);
	assert.equal(poseAtFrame(TELESCOPE_DURATION + 5), telescopeFrames[TELESCOPE_DURATION - 1]);
	assert.equal(poseAtFrame(42), telescopeFrames[42]);
});

test("no drawing is held for longer than ten frames", () => {
	let run = 1;
	for (let frame = 1; frame < TELESCOPE_DURATION; frame++) {
		run = telescopeFrames[frame] === telescopeFrames[frame - 1] ? run + 1 : 1;
		assert.ok(run <= 10, `frame ${frame} holds one drawing for ${run} frames`);
	}
});

test("the arm splits rebuild the shared layers exactly", () => {
	assert.equal(flat([shoulderRight, forearmRight]), flat([armSideRight]));
	assert.equal(flat([sleeveDown(SLEEVE_HOME), fistDown(FIST_HOME)]), flat([forearmRight]));
	assert.equal(flat([sleeveRaised(SLEEVE_FULL)]), flat([sleeveTelescopeRight]));
});

test("both arms behind his back redraw the shared arms layer", () => {
	assert.equal(flat([armBehindLeft, armBehindRight]), flat([arms]));
});

test("the reach starts on the base pose and lands on the side pose", () => {
	assert.equal(flat(reachStages[0]), flat(basePose));
	assert.equal(flat(reachStages[reachStages.length - 1]), flat(telescopeSidePose));
});

test("the raise lands on the spyglass-to-eye pose", () => {
	assert.equal(flat(raiseStages[raiseStages.length - 1]), flat(telescopeEyePose));
});

test("the arm travels instead of teleporting", () => {
	reachStages.slice(1).forEach((pose, index) => {
		const moved = inkCount(pose) - inkCount(reachStages[index]);
		assert.ok(Math.abs(moved) < 120, `reach drawing ${index + 1} changes ${moved} pixels of ink at once`);
	});
});

test("the arrival composes at every stage and ends with the lens filling the frame", () => {
	arrivalStages.forEach((pose, index) => {
		assert.doesNotThrow(() => compose(pose), `arrival stage ${index} does not compose`);
	});
	const filled = compose(arrivalStages[arrivalStages.length - 1]);
	assert.ok(
		filled.every((row) => !row.includes(" ")),
		"the last lens stage leaves transparent pixels in the frame",
	);
	assert.ok(filled[0].split("").every((cell) => cell === "#"), "the barrel does not black out the top row");
});

test("the barrel foreshortens as he turns the objective toward us", () => {
	const barrelArea = (pose: Pose): number => {
		const tube = pose.filter((layer) => layer.name === "spyglassEndOn");
		assert.equal(tube.length, 1, "the turning drawing has no single barrel layer");
		return compose([tube[0]]).join("").replace(/ /g, "").length;
	};
	const areas = [...foreshortenStages, growStages[0]].map(barrelArea);
	areas.slice(1).forEach((area, index) => {
		assert.ok(area < areas[index], `the barrel grew from ${areas[index]} to ${area} while turning end-on`);
	});
});

test("the lens grows concentrically about the middle of the frame", () => {
	// Every stage's glass must be symmetric about the canvas centre, so the lens swells
	// toward us rather than sliding in from the side.
	growStages.forEach((pose, index) => {
		const glass = pose.filter((layer) => layer.name === "lensGlass");
		assert.equal(glass.length, 1, `grow stage ${index} has no single glass layer`);
		compose([glass[0]]).forEach((row, y) => {
			assert.equal(
				row,
				row.split("").reverse().join(""),
				`the glass on grow stage ${index} is off centre at row ${y}`,
			);
		});
	});
});

test("the last arrival drawing is exactly the close-up, so the hand-off is seamless", () => {
	assert.equal(flat(arrivalStages[arrivalStages.length - 1]), flat(lensPose({ dx: 0, dy: 0, lid: 10 })));
});

test("a shut eye draws a lash line and an open one draws an iris", () => {
	assert.ok(
		inkCount([craigEye({ dx: 0, dy: 0, lid: 10 })]) > inkCount([craigEye({ dx: 0, dy: 0, lid: 0 })]),
		"the open eye should carry more ink than the lash line",
	);
	assert.ok(inkCount([craigEye({ dx: 0, dy: 0, lid: 0 })]) > 0, "the shut eye draws nothing");
});
