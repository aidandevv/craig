import { basePose } from "../../base.ts";
import type { Pose } from "../../layer.ts";
import { armSideRight, handTelescopeRight, sleeveTelescopeRight } from "../../layers/arms.ts";
import { browsSquint } from "../../layers/brows.ts";
import { eyesSquintLeft } from "../../layers/eyes.ts";
import { telescopeCollapsed, telescopeRaised } from "../../layers/telescope.ts";
import { mirrorPose } from "../../mirror.ts";
import { fist, hangingSleeve, raisedSleeve, restArm } from "./arm.ts";
import { peekPose } from "./pose.ts";
import { glassLens, glassShort, glassTipping, raised, stowedDrawn, upright } from "./prop.ts";

/** One drawing, held for a few frames. Nothing is tweened between them. */
type Beat = {
	readonly hold: number;
	readonly pose: Pose;
};

// He brings the spyglass out from behind his back and lets it down into his fist.
const handOut = peekPose({ hand: fist(46, 58), name: "handOut", sleeve: restArm });
const gripped = peekPose({ name: "gripped", sleeve: armSideRight });
const sliding = peekPose({ name: "sliding", sleeve: armSideRight, stowed: stowedDrawn(8) });
const nearlyOut = peekPose({ name: "nearlyOut", sleeve: armSideRight, stowed: stowedDrawn(11) });

/** The same layers, in the same order, as the shared telescope-side pose. */
const atSide = peekPose({ name: "atSide", sleeve: armSideRight, stowed: telescopeCollapsed });

// The climb to his eye: upright in his fist, tipping over, then carried up level.
const lift1 = peekPose({ hand: fist(41, 51), name: "lift1", sleeve: hangingSleeve(53), stowed: upright(52) });
const lift2 = peekPose({ glass: glassTipping, hand: fist(41, 45), name: "lift2", sleeve: hangingSleeve(49) });
const lift3 = peekPose({ glass: raised(18), hand: fist(41, 42), name: "lift3", sleeve: hangingSleeve(49) });
const lift4 = peekPose({ glass: raised(12), hand: fist(41, 36), name: "lift4", sleeve: raisedSleeve(42) });
const lift5 = peekPose({ glass: raised(6), hand: fist(41, 30), name: "lift5", sleeve: raisedSleeve(36) });

/** The same layers, in the same order, as the shared telescope-eye pose. */
const atEye = peekPose({
	brows: browsSquint,
	eyes: eyesSquintLeft,
	glass: telescopeRaised,
	hand: handTelescopeRight,
	name: "atEye",
	sleeve: sleeveTelescopeRight,
});

// The peek itself: the barrel swings round to the viewer and on to his other side.
const shortGlass = peekPose({
	brows: browsSquint,
	eyes: eyesSquintLeft,
	glass: glassShort,
	hand: handTelescopeRight,
	name: "shortGlass",
	sleeve: sleeveTelescopeRight,
});

const centre = peekPose({
	brows: browsSquint,
	eyes: eyesSquintLeft,
	glass: glassLens,
	hand: handTelescopeRight,
	name: "centre",
	sleeve: sleeveTelescopeRight,
});

const atEyeLeft = mirrorPose(atEye);
const shortGlassLeft = mirrorPose(shortGlass);
const centreLeft = mirrorPose(centre);

const beats: Beat[] = [];

const add = (pose: Pose, hold: number): void => {
	beats.push({ hold, pose });
};

/** One look right, round past the viewer, a look left, and back again. */
const swing = (): void => {
	add(shortGlass, 3);
	add(centre, 3);
	add(centreLeft, 3);
	add(shortGlassLeft, 3);
	add(atEyeLeft, 4);
	add(shortGlassLeft, 3);
	add(centreLeft, 3);
	add(centre, 3);
	add(shortGlass, 3);
};

// 1. He stands there, hands behind his back.
add(basePose, 10);

// 2. He draws the spyglass out and lets it hang at his side.
add(handOut, 4);
add(gripped, 3);
add(sliding, 3);
add(nearlyOut, 3);
add(atSide, 3);

// 3. He raises it to his eye.
add(lift1, 4);
add(lift2, 3);
add(lift3, 3);
add(lift4, 3);
add(lift5, 4);

// 4. He peeks one way, past the viewer, the other way, and back. Three times over.
for (let round = 0; round < 3; round++) {
	add(atEye, 4);
	swing();
}
add(atEye, 4);

// 5. He lowers it, stows it behind his back, and stands as he began.
add(lift5, 3);
add(lift4, 3);
add(lift3, 3);
add(lift2, 3);
add(lift1, 3);
add(atSide, 3);
add(nearlyOut, 3);
add(sliding, 2);
add(gripped, 2);
add(handOut, 2);
add(basePose, 10);

const expand = (list: readonly Beat[]): readonly Pose[] => {
	const out: Pose[] = [];
	for (const beat of list) {
		for (let held = 0; held < beat.hold; held++) {
			out.push(beat.pose);
		}
	}
	return out;
};

/** Craig's peek loop, one pose per frame. */
export const peekFrames: readonly Pose[] = expand(beats);

export const peekFps = 24;

export const peekDuration = peekFrames.length;

/** The pose for a frame, clamped so the composition's edges never fall off the plan. */
export const peekFrame = (frame: number): Pose => {
	const index = Math.min(Math.max(Math.floor(frame), 0), peekFrames.length - 1);
	return peekFrames[index];
};
