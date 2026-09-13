import { CLEAR, INK, PAPER } from "../../layer.ts";
import type { Layer } from "../../layer.ts";
import { distanceTo, paintLayer } from "./raster.ts";

/** Where the close-up lens settles: the middle of the canvas. */
export const LENS_CX = 28;
export const LENS_CY = 40;
/** Outer radius of the settled lens, rim included. */
export const LENS_RADIUS = 24;
/** Ink thickness of the settled lens rim. */
export const LENS_RIM = 3;

/** Middle of the eye drawn inside the lens. */
const EYE_CX = 28;
const EYE_CY = 42;
/** Half-width of the open eye. */
const EYE_RX = 16.5;
/** Half-height of the open eye, at its widest. */
export const EYE_RY = 10;
const EYE_RIM = 1.5;
const IRIS_RADIUS = 6;

/** How far the iris can travel before the lid would clip it. */
export const IRIS_REACH_X = 8;
export const IRIS_REACH_Y = 2;

export type Glance = {
	/** Iris offset from the middle of the eye, in art pixels. */
	readonly dx: number;
	readonly dy: number;
	/** Half-height of the lid opening; 0 draws the closed lash line. */
	readonly lid: number;
};

const irisPaint = (x: number, y: number, irisX: number, irisY: number): string | null => {
	const d = distanceTo(x, y, irisX, irisY);
	if (d > IRIS_RADIUS) {
		return null;
	}
	if (distanceTo(x, y, irisX - 1.5, irisY - 1.5) <= 1.2) {
		return PAPER;
	}
	return d > 3.5 && d <= 4.5 ? PAPER : INK;
};

/** The lash line of a shut eye: a heavy stroke that droops at both corners. */
const lashLayer = (): Layer =>
	paintLayer("craigEyeShut", (x, y) => {
		const dx = x + 0.5 - EYE_CX;
		if (Math.abs(dx) > EYE_RX) {
			return CLEAR;
		}
		const droop = Math.abs(dx) > EYE_RX - 4 ? 1 : 0;
		return Math.abs(y + 0.5 - (EYE_CY + droop)) <= 1 ? INK : CLEAR;
	});

/**
 * Craig's eye as seen through the spyglass: an ink-outlined lid with the iris inside.
 * The whole thing is one rasterised layer so the iris is always clipped by the lid.
 */
export const craigEye = ({ dx, dy, lid }: Glance): Layer => {
	if (lid <= 0) {
		return lashLayer();
	}
	const irisX = EYE_CX + dx;
	const irisY = EYE_CY + dy;
	const innerRx = EYE_RX - EYE_RIM;
	const innerRy = lid - EYE_RIM;
	return paintLayer(`craigEye${dx}_${dy}_${lid}`, (x, y) => {
		const ox = (x + 0.5 - EYE_CX) / EYE_RX;
		const oy = (y + 0.5 - EYE_CY) / lid;
		if (Math.hypot(ox, oy) > 1) {
			return CLEAR;
		}
		if (innerRy <= 0) {
			return INK;
		}
		const ix = (x + 0.5 - EYE_CX) / innerRx;
		const iy = (y + 0.5 - EYE_CY) / innerRy;
		if (Math.hypot(ix, iy) > 1) {
			return INK;
		}
		return irisPaint(x, y, irisX, irisY) ?? PAPER;
	});
};

const BROW_HALF_WIDTH = 14;
const BROW_THICKNESS = 4;
const BROW_ARC = 3;

/** His brow above the close-up eye: an arched bar with bristles along its top. */
export const craigBrow = (dy: number): Layer =>
	paintLayer(`craigBrow${dy}`, (x, y) => {
		const dx = x + 0.5 - EYE_CX;
		if (Math.abs(dx) > BROW_HALF_WIDTH) {
			return CLEAR;
		}
		const top = 24 + dy + Math.round(BROW_ARC * (dx / BROW_HALF_WIDTH) ** 2);
		const bristle = x % 3 === 0 ? 1 : 0;
		return y >= top - bristle && y < top + BROW_THICKNESS ? INK : CLEAR;
	});
