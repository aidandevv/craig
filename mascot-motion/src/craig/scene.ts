import type { Canvas } from "./compose.ts";
import { shiftPose } from "./idle.ts";
import type { Pose } from "./layer.ts";
import { crowsNest, mast, seagulls } from "./layers/scene.ts";
import { hatMark } from "./layers/mark.ts";
import { gullLevel, gullUp, waveTile } from "./layers/ocean.ts";
import { shadeEyesPose } from "./poses.ts";

/** A scene is wider and taller than Craig's own canvas, with him standing somewhere inside it. */
export const CROWS_NEST_CANVAS: Canvas = { height: 96, width: 112 };

/** Where Craig's own 56x80 canvas sits inside the scene. */
const CRAIG_AT = { x: 28, y: 8 };

/**
 * Craig up in the crow's nest with his hand over his brow, gulls crossing behind him.
 * The basket is drawn last so it covers him from the hip down, as if he stands in it.
 */
export const crowsNestScene: Pose = [seagulls, mast, ...shiftPose(shadeEyesPose, CRAIG_AT.x, CRAIG_AT.y), crowsNest];

/** The ocean tile is its own small canvas, drawn once and repeated across the footer. */
export const WAVE_TILE_CANVAS: Canvas = { height: 28, width: 64 };
export const waveTilePose: Pose = [waveTile];

/** Gull frames, small enough to sit in a page margin. */
export const GULL_CANVAS: Canvas = { height: 6, width: 15 };
export const gullUpPose: Pose = [gullUp];
export const gullLevelPose: Pose = [gullLevel];

/** The extension icon: one 16x16 drawing, scaled by whole numbers to every size Chrome asks for. */
export const MARK_CANVAS: Canvas = { height: 16, width: 16 };
export const hatMarkPose: Pose = [hatMark];
