import type { Pose } from "../../layer.ts";
import type { Glance } from "./lens.ts";
import { arrivalStages, lensPose, raiseStages, reachStages } from "./poses.ts";

export const TELESCOPE_FPS = 24;

/** One drawing and how many frames it is held for. Nothing tweens; every change is a drawing. */
type Beat = {
	readonly frames: number;
	readonly pose: Pose;
};

const beat = (pose: Pose, frames: number): Beat => ({ frames, pose });
const beatEach = (poses: readonly Pose[], frames: number): Beat[] => poses.map((pose) => beat(pose, frames));
/** Reverses a run of drawings and drops the first, which the previous beat already ended on. */
const rewind = (poses: readonly Pose[]): readonly Pose[] => [...poses].reverse().slice(1);

type GlanceBeat = Glance & { readonly frames: number };

/** His eye hunting across the view: two frames to dart, then a longer hold to look. */
const searching: readonly GlanceBeat[] = [
	{ dx: 0, dy: 0, frames: 6, lid: 10 },
	{ dx: -4, dy: -1, frames: 2, lid: 10 },
	{ dx: -8, dy: -1, frames: 8, lid: 10 },
	{ dx: -6, dy: 2, frames: 8, lid: 10 },
	{ dx: 0, dy: 0, frames: 2, lid: 10 },
	{ dx: 7, dy: -1, frames: 10, lid: 10 },
	{ dx: 8, dy: 2, frames: 6, lid: 10 },
	{ dx: 2, dy: 0, frames: 2, lid: 10 },
	{ dx: -3, dy: -2, frames: 10, lid: 10 },
	{ dx: 1, dy: 0, frames: 2, lid: 10 },
	{ dx: 5, dy: 1, frames: 8, lid: 10 },
	{ dx: 2, dy: 0, frames: 2, lid: 10 },
	{ dx: 0, dy: 0, frames: 6, lid: 10 },
];

/** A blink: shut fast, hold, then open again. */
const blinking: readonly GlanceBeat[] = [
	{ dx: 0, dy: 0, frames: 2, lid: 8 },
	{ dx: 0, dy: 0, frames: 2, lid: 3 },
	{ dx: 0, dy: 0, frames: 3, lid: 0 },
	{ dx: 0, dy: 0, frames: 2, lid: 3 },
	{ dx: 0, dy: 0, frames: 2, lid: 8 },
	{ dx: 0, dy: 0, frames: 8, lid: 10 },
];

const glanceBeats = (glances: readonly GlanceBeat[]): Beat[] =>
	glances.map(({ dx, dy, frames, lid }) => beat(lensPose({ dx, dy, lid }), frames));

const beats: readonly Beat[] = [
	// 1. He stands at ease.
	beat(reachStages[0], 8),
	// 2. The right arm swings out with the collapsed spyglass.
	...beatEach(reachStages.slice(1), 3),
	// 3. Up his side and over to his eye.
	...beatEach(raiseStages, 3),
	// 4. He turns the objective toward us and the lens swells until it fills the frame.
	...beatEach(arrivalStages, 3),
	// 5. He searches, then blinks.
	...glanceBeats(searching),
	...glanceBeats(blinking),
	// 6. The lens shrinks back onto the objective, which unforeshortens into the barrel.
	...beatEach(rewind(arrivalStages), 2),
	// 7. He lowers the spyglass and puts it away behind his back.
	...beatEach(rewind(raiseStages), 2),
	...beatEach(rewind(reachStages), 2),
	beat(reachStages[0], 4),
];

/** Every frame of the loop, as a drawing. The last frame returns to the first. */
export const telescopeFrames: readonly Pose[] = beats.flatMap(({ frames, pose }) =>
	Array.from({ length: frames }, () => pose),
);

export const TELESCOPE_DURATION = telescopeFrames.length;

/** The drawing shown on a given frame, clamped so a stray frame never reads off the end. */
export const poseAtFrame = (frame: number): Pose =>
	telescopeFrames[Math.min(Math.max(frame, 0), TELESCOPE_DURATION - 1)];
