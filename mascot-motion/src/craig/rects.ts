import { INK, PAPER } from "./layer.ts";
import type { Grid, Paint } from "./layer.ts";

/** A 1 px tall run of same-colored pixels. */
export type Rect = {
	readonly x: number;
	readonly y: number;
	readonly width: number;
	readonly paint: Paint;
};

/** Merges each row's runs of same-colored pixels into rectangles, skipping transparent cells. */
export const toRects = (grid: Grid): Rect[] => {
	const rects: Rect[] = [];
	grid.forEach((row, y) => {
		let x = 0;
		while (x < row.length) {
			const cell = row[x];
			let end = x + 1;
			while (end < row.length && row[end] === cell) {
				end++;
			}
			if (cell === INK || cell === PAPER) {
				rects.push({ paint: cell, width: end - x, x, y });
			}
			x = end;
		}
	});
	return rects;
};
