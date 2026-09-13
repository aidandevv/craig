import { basePose } from "../../base.ts";
import type { Pose } from "../../layer.ts";
import { brows } from "../../layers/brows.ts";
import { eyesLookRight } from "../../layers/eyes.ts";
import { armLift, armTipped } from "./arm.ts";
import { browsLifted, gaze } from "./face.ts";
import { lookoutPose } from "./pose.ts";

/** One drawing, held for a few frames. Nothing is tweened between them. */
type Beat = {
	readonly hold: number;
	readonly pose: Pose;
};

/** The lookout arm at the top of its climb: hand flat over the brow. */
const shading = armLift[armLift.length - 1];

const beats: Beat[] = [];

const add = (pose: Pose, hold: number): void => {
	beats.push({ hold, pose });
};

// 1. He stands there, hands behind his back.
add(basePose, 8);

// 2. The arm comes up: hand out from behind his back, up his side, then flat over his brow.
const raiseHolds: readonly number[] = [3, 3, 3, 3, 3, 3, 2, 2, 4];
for (let step = 0; step < raiseHolds.length; step++) {
	const last = step === raiseHolds.length - 1;
	add(lookoutPose(armLift[step + 1], last ? eyesLookRight : gaze(0), brows), raiseHolds[step]);
}

// 3. He searches: the gaze sweeps out to the horizon and back, twice, with the brow and hand shifting.
add(lookoutPose(shading, gaze(0), brows), 4);
add(lookoutPose(shading, gaze(-1), brows), 4);
add(lookoutPose(shading, gaze(0), brows), 3);
add(lookoutPose(shading, gaze(1), brows), 3);
add(lookoutPose(shading, gaze(2), brows), 4);
add(lookoutPose(shading, gaze(3), brows), 4);
add(lookoutPose(shading, gaze(3), browsLifted), 4);
add(lookoutPose(armTipped, gaze(3), browsLifted), 4);
add(lookoutPose(armTipped, gaze(2), brows), 3);
add(lookoutPose(armTipped, gaze(1), brows), 3);
add(lookoutPose(shading, gaze(0), brows), 4);
add(lookoutPose(shading, gaze(-1), brows), 4);
add(lookoutPose(shading, gaze(0), brows), 3);
add(lookoutPose(shading, gaze(1), brows), 3);
add(lookoutPose(shading, gaze(2), brows), 4);
add(lookoutPose(shading, gaze(3), brows), 4);
add(lookoutPose(shading, gaze(1), brows), 3);

// 4. The arm drops back down the way it came, his eyes levelling off as it goes.
const lowerHolds: readonly number[] = [2, 2, 2, 2, 2, 2, 2, 3];
for (let step = 0; step < lowerHolds.length; step++) {
	add(lookoutPose(armLift[armLift.length - 2 - step], gaze(step === 0 ? 1 : 0), brows), lowerHolds[step]);
}

// 5. Back to the base pose, exactly, so the loop closes.
add(basePose, 8);

const expand = (list: readonly Beat[]): readonly Pose[] => {
	const out: Pose[] = [];
	for (const beat of list) {
		for (let held = 0; held < beat.hold; held++) {
			out.push(beat.pose);
		}
	}
	return out;
};

/** Craig's lookout loop, one pose per frame. */
export const lookoutFrames: readonly Pose[] = expand(beats);

export const lookoutFps = 24;

export const lookoutDuration = lookoutFrames.length;

/** The pose for a frame, clamped so the composition's edges never fall off the plan. */
export const lookoutFrame = (frame: number): Pose => {
	const index = Math.min(Math.max(Math.floor(frame), 0), lookoutFrames.length - 1);
	return lookoutFrames[index];
};
