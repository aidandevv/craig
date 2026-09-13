import { basePose } from "./base.ts";
import type { Layer, Pose } from "./layer.ts";
import { eyesClosed, eyesHalf } from "./layers/eyes.ts";

/** Frames per second for the idle loop: slow enough to read as stop motion. */
export const idleFps = 8;

/** Moves a whole pose by whole pixels, so every drawing stays on the pixel grid. */
export const shiftPose = (pose: Pose, dx: number, dy: number): Pose =>
	pose.map((layer) => ({ ...layer, x: layer.x + dx, y: layer.y + dy }));

/** Moves only the named layers, for a breath or a tipped hat. */
export const nudgeLayers = (pose: Pose, names: readonly string[], dx: number, dy: number): Pose =>
	pose.map((layer) => (names.indexOf(layer.name) === -1 ? layer : { ...layer, x: layer.x + dx, y: layer.y + dy }));

/** Replaces the layer with this name and leaves the rest of the pose alone. */
export const swapLayer = (pose: Pose, name: string, replacement: Layer): Pose =>
	pose.map((layer) => (layer.name === name ? replacement : layer));

/**
 * Craig fills the canvas from row 0 to row 79, so nothing can shift downward as a whole
 * without falling off it. The breath moves his head and hat together instead.
 */
const FACE = ["head", "mouth", "mustache", "nose", "eyes", "brows"];
const HEAD = [...FACE, "hat", "badge"];

const breathing = nudgeLayers(basePose, HEAD, 0, 1);
const halfShut = swapLayer(basePose, "eyes", eyesHalf);
const shut = swapLayer(basePose, "eyes", eyesClosed);
/** His head dips while the hat holds its place, so the hat reads as lifted. */
const tipLow = nudgeLayers(basePose, FACE, 0, 1);
const tipHigh = nudgeLayers(nudgeLayers(basePose, FACE, 0, 2), ["hat", "badge"], 1, 0);

const hold = (pose: Pose, frames: number): Pose[] => Array.from({ length: frames }, () => pose);

/**
 * One drawing per cell: 48 cells at 8 fps, six seconds. He breathes twice, blinks once,
 * tips his hat once, and every stretch returns to the same resting pose.
 */
export const idlePoses: readonly Pose[] = [
	...hold(basePose, 12),
	...hold(breathing, 4),
	...hold(basePose, 4),
	halfShut,
	shut,
	shut,
	halfShut,
	...hold(basePose, 8),
	...hold(breathing, 4),
	...hold(basePose, 4),
	tipLow,
	tipHigh,
	tipHigh,
	tipLow,
	...hold(basePose, 4),
];

export const idleDuration = idlePoses.length;

/** The drawing for a frame, looping forever. */
export const idlePoseAt = (frame: number): Pose => idlePoses[((frame % idleDuration) + idleDuration) % idleDuration];
