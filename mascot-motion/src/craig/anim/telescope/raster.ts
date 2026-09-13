import { CANVAS_HEIGHT, CANVAS_WIDTH, CLEAR, INK, PAPER } from "../../layer.ts";
import type { Layer } from "../../layer.ts";

/**
 * Builds a whole-canvas layer by asking for one character per art pixel. Every shape
 * in this folder is rasterised, so circles and tubes land on whole pixels with hard
 * edges instead of being drawn as SVG curves.
 */
export const paintLayer = (name: string, paint: (x: number, y: number) => string): Layer => {
	const rows: string[] = [];
	for (let y = 0; y < CANVAS_HEIGHT; y++) {
		let row = "";
		for (let x = 0; x < CANVAS_WIDTH; x++) {
			row += paint(x, y);
		}
		rows.push(row);
	}
	return { name, rows, x: 0, y: 0 };
};

/** Distance from an art pixel's centre to a point. */
export const distanceTo = (x: number, y: number, cx: number, cy: number): number =>
	Math.hypot(x + 0.5 - cx, y + 0.5 - cy);

/** Distance from an art pixel's centre to a line segment. */
export const distanceToSegment = (
	x: number,
	y: number,
	ax: number,
	ay: number,
	bx: number,
	by: number,
): number => {
	const px = x + 0.5;
	const py = y + 0.5;
	const dx = bx - ax;
	const dy = by - ay;
	const lengthSquared = dx * dx + dy * dy;
	const t = lengthSquared === 0 ? 0 : Math.min(1, Math.max(0, ((px - ax) * dx + (py - ay) * dy) / lengthSquared));
	return Math.hypot(px - (ax + t * dx), py - (ay + t * dy));
};

export type Disc = {
	readonly cx: number;
	readonly cy: number;
	readonly radius: number;
	/** Ink border thickness, in art pixels. */
	readonly rim: number;
};

/** A filled circle: paper glass inside an ink rim, transparent beyond. */
export const discLayer = (name: string, { cx, cy, radius, rim }: Disc): Layer =>
	paintLayer(name, (x, y) => {
		const d = distanceTo(x, y, cx, cy);
		if (d > radius) {
			return CLEAR;
		}
		return d > radius - rim ? INK : PAPER;
	});

/** Solid ink everywhere outside a circle: the inside of the spyglass barrel. */
export const vignetteLayer = (name: string, cx: number, cy: number, radius: number): Layer =>
	paintLayer(name, (x, y) => (distanceTo(x, y, cx, cy) > radius ? INK : CLEAR));

export type Tube = {
	readonly ax: number;
	readonly ay: number;
	readonly bx: number;
	readonly by: number;
	readonly halfWidth: number;
	readonly rim: number;
	/** Ink bands across the barrel, as fractions of the run from a to b. */
	readonly bands?: readonly number[];
};

/** A straight barrel between two points: paper inside, ink outline and ink bands. */
export const tubeLayer = (name: string, { ax, ay, bands = [], bx, by, halfWidth, rim }: Tube): Layer => {
	const length = Math.hypot(bx - ax, by - ay);
	return paintLayer(name, (x, y) => {
		const d = distanceToSegment(x, y, ax, ay, bx, by);
		if (d > halfWidth) {
			return CLEAR;
		}
		if (d > halfWidth - rim) {
			return INK;
		}
		const along = length === 0 ? 0 : ((x + 0.5 - ax) * (bx - ax) + (y + 0.5 - ay) * (by - ay)) / length;
		return bands.some((band) => Math.abs(along - band * length) < 0.75) ? INK : PAPER;
	});
};
