import { CANVAS_HEIGHT, CANVAS_WIDTH, CLEAR, INK, PAPER } from "./layer.ts";
import type { Grid, Layer, Pose } from "./layer.ts";

/** The area a pose is stacked onto: Craig's own canvas by default, or a larger scene around him. */
export type Canvas = {
	readonly width: number;
	readonly height: number;
};

const CRAIG_CANVAS: Canvas = { height: CANVAS_HEIGHT, width: CANVAS_WIDTH };

const validate = ({ name, rows, x, y }: Layer, canvas: Canvas): void => {
	if (rows.length === 0 || rows[0].length === 0) {
		throw new Error(`Layer "${name}" is empty.`);
	}
	const width = rows[0].length;
	rows.forEach((row, index) => {
		if (row.length !== width) {
			throw new Error(`Layer "${name}" row ${index} is ${row.length} wide; expected ${width}.`);
		}
		for (const char of row) {
			if (char !== INK && char !== PAPER && char !== CLEAR) {
				throw new Error(`Layer "${name}" row ${index} has unknown character "${char}".`);
			}
		}
	});
	const inside =
		Number.isInteger(x) &&
		Number.isInteger(y) &&
		x >= 0 &&
		y >= 0 &&
		x + width <= canvas.width &&
		y + rows.length <= canvas.height;
	if (!inside) {
		throw new Error(
			`Layer "${name}" (${width}x${rows.length} at ${x},${y}) falls outside the ${canvas.width}x${canvas.height} canvas.`,
		);
	}
};

/** Stacks a pose bottom to top. Ink and paper cover lower layers; transparent cells do not. */
export const compose = (pose: Pose, canvas: Canvas = CRAIG_CANVAS): Grid => {
	const cells = Array.from({ length: canvas.height }, () => new Array<string>(canvas.width).fill(CLEAR));
	for (const layer of pose) {
		validate(layer, canvas);
		layer.rows.forEach((row, dy) => {
			for (let dx = 0; dx < row.length; dx++) {
				if (row[dx] !== CLEAR) {
					cells[layer.y + dy][layer.x + dx] = row[dx];
				}
			}
		});
	}
	return cells.map((row) => row.join(""));
};

/** One line per row, with transparent cells shown as `_`. For review and snapshots only; `_` is not a layer character. */
export const gridToText = (grid: Grid): string => grid.map((row) => row.replace(/ /g, "_")).join("\n") + "\n";
