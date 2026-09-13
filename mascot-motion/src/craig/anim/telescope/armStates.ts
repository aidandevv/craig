import type { Layer } from "../../layer.ts";
import { armBehindLeft, armSideRight, sleeveTelescopeRight } from "../../layers/arms.ts";
import { mirrorLayer } from "../../mirror.ts";

/** Moves a layer without redrawing it, for in-between frames. */
export const shift = (layer: Layer, dx: number, dy: number): Layer => ({
	...layer,
	x: layer.x + dx,
	y: layer.y + dy,
});

/**
 * His right arm clasped behind his back. `arms` draws both at once, so the right one is
 * the mirror of the left: if that shared layer is redrawn, this follows it.
 */
export const armBehindRight: Layer = { ...mirrorLayer(armBehindLeft), name: "armBehindRight" };

/** Rows of `armSideRight` that are shoulder rather than arm. */
const SHOULDER_ROWS = 4;
/** Rows of the lower arm that are straight sleeve, before the fist starts. */
const SLEEVE_ROWS = 11;

/** The shoulder of the side pose, which stays put once the arm is out. */
export const shoulderRight: Layer = {
	name: "shoulderRight",
	rows: armSideRight.rows.slice(0, SHOULDER_ROWS),
	x: armSideRight.x,
	y: armSideRight.y,
};

/** Sleeve and fist of the side pose as one travelling unit, split off below the shoulder. */
export const forearmRight: Layer = {
	name: "forearmRight",
	rows: armSideRight.rows.slice(SHOULDER_ROWS),
	x: armSideRight.x,
	y: armSideRight.y + SHOULDER_ROWS,
};

/**
 * The hanging sleeve cut to `rows`, anchored at the shoulder. Pair it with `fistDown` to
 * bend his elbow: the sleeve shortens while the fist climbs, so the arm never stretches.
 */
export const sleeveDown = (rows: number): Layer => ({
	name: `sleeveDown${rows}`,
	rows: forearmRight.rows.slice(0, rows),
	x: forearmRight.x,
	y: forearmRight.y,
});

/** His closed fist at the end of the hanging sleeve, placed by its top row. */
export const fistDown = (top: number): Layer => ({
	name: `fistDown${top}`,
	rows: forearmRight.rows.slice(SLEEVE_ROWS),
	x: forearmRight.x,
	y: top,
});

/** Sleeve rows and fist row that put `sleeveDown` and `fistDown` back at their home position. */
export const SLEEVE_HOME = SLEEVE_ROWS;
export const FIST_HOME = forearmRight.y + SLEEVE_ROWS;

/** The raised sleeve cut to `rows`, kept anchored at the shoulder so only its top travels. */
export const sleeveRaised = (rows: number): Layer => ({
	name: `sleeveRaised${rows}`,
	rows: sleeveTelescopeRight.rows.slice(sleeveTelescopeRight.rows.length - rows),
	x: sleeveTelescopeRight.x,
	y: sleeveTelescopeRight.y + (sleeveTelescopeRight.rows.length - rows),
});

/** How many rows the full raised sleeve has. */
export const SLEEVE_FULL = sleeveTelescopeRight.rows.length;
