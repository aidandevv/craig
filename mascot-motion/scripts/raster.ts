import { INK, PAPER } from "../src/craig/layer.ts";
import type { Grid } from "../src/craig/layer.ts";
import type { RgbaImage } from "./png.ts";

/** An 8-bit RGBA color. */
export type Rgba = readonly [number, number, number, number];

export type Colors = {
	readonly ink: Rgba;
	readonly paper: Rgba;
	/** Fills transparent cells. */
	readonly background: Rgba;
};

/** One horizontal strip of a contact sheet. */
export type Band = {
	readonly background: Rgba;
	readonly images: readonly RgbaImage[];
};

/** A width x height image filled with one color. */
export const blank = (width: number, height: number, color: Rgba): RgbaImage => {
	const data = new Uint8Array(width * height * 4);
	for (let offset = 0; offset < data.length; offset += 4) {
		data.set(color, offset);
	}
	return { data, height, width };
};

/** Paints a grid with every art pixel as a scale x scale block. */
export const rasterize = (grid: Grid, scale: number, colors: Colors): RgbaImage => {
	if (!Number.isInteger(scale) || scale < 1) {
		throw new Error(`Scale must be a positive integer, got ${scale}.`);
	}
	const image = blank(grid[0].length * scale, grid.length * scale, colors.background);
	grid.forEach((row, y) => {
		for (let x = 0; x < row.length; x++) {
			const color = row[x] === INK ? colors.ink : row[x] === PAPER ? colors.paper : null;
			if (color === null) {
				continue;
			}
			for (let py = y * scale; py < (y + 1) * scale; py++) {
				for (let px = x * scale; px < (x + 1) * scale; px++) {
					image.data.set(color, (py * image.width + px) * 4);
				}
			}
		}
	});
	return image;
};

/** Copies source into target with its top-left corner at (left, top). Overwrites; no blending. */
export const paste = (target: RgbaImage, source: RgbaImage, left: number, top: number): void => {
	for (let y = 0; y < source.height; y++) {
		const start = y * source.width * 4;
		target.data.set(source.data.subarray(start, start + source.width * 4), ((top + y) * target.width + left) * 4);
	}
};

/** Stacks bands vertically. Within a band, images sit side by side, bottom-aligned, with `padding` px around and between them. */
export const sheet = (bands: readonly Band[], padding: number): RgbaImage => {
	const bandWidth = (band: Band): number => band.images.reduce((sum, image) => sum + image.width + padding, padding);
	const bandHeight = (band: Band): number => Math.max(...band.images.map((image) => image.height)) + padding * 2;
	const width = Math.max(...bands.map(bandWidth));
	const result = blank(
		width,
		bands.reduce((sum, band) => sum + bandHeight(band), 0),
		[0, 0, 0, 0],
	);
	let top = 0;
	for (const band of bands) {
		const height = bandHeight(band);
		paste(result, blank(width, height, band.background), 0, top);
		let left = padding;
		for (const image of band.images) {
			paste(result, image, left, top + height - padding - image.height);
			left += image.width + padding;
		}
		top += height;
	}
	return result;
};
