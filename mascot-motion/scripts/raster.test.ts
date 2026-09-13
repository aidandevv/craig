import assert from "node:assert/strict";
import { test } from "node:test";
import type { RgbaImage } from "./png.ts";
import { rasterize, sheet } from "./raster.ts";
import type { Colors, Rgba } from "./raster.ts";

const INK: Rgba = [0, 0, 0, 255];
const PAPER: Rgba = [255, 255, 255, 255];
const CLEAR: Rgba = [0, 0, 0, 0];
const colors: Colors = { background: CLEAR, ink: INK, paper: PAPER };

const pixel = (image: RgbaImage, x: number, y: number): number[] => {
	const at = (y * image.width + x) * 4;
	return [...image.data.subarray(at, at + 4)];
};

test("rasterize paints ink, paper, and background", () => {
	const image = rasterize(["#. "], 1, colors);
	assert.equal(image.width, 3);
	assert.equal(image.height, 1);
	assert.deepEqual(pixel(image, 0, 0), [...INK]);
	assert.deepEqual(pixel(image, 1, 0), [...PAPER]);
	assert.deepEqual(pixel(image, 2, 0), [...CLEAR]);
});

test("rasterize turns each art pixel into a scale x scale block", () => {
	const image = rasterize(["#.", " #"], 3, colors);
	assert.equal(image.width, 6);
	assert.equal(image.height, 6);
	assert.deepEqual(pixel(image, 2, 2), [...INK]);
	assert.deepEqual(pixel(image, 3, 2), [...PAPER]);
	assert.deepEqual(pixel(image, 2, 3), [...CLEAR]);
	assert.deepEqual(pixel(image, 5, 5), [...INK]);
});

test("rasterize rejects scales that aren't positive integers", () => {
	assert.throws(() => rasterize(["#"], 1.5, colors), /positive integer/);
	assert.throws(() => rasterize(["#"], 0, colors), /positive integer/);
});

test("sheet bottom-aligns images inside a padded band", () => {
	const RED: Rgba = [255, 0, 0, 255];
	const tall = rasterize(["#", "#"], 1, colors);
	const short = rasterize(["."], 1, colors);
	const result = sheet([{ background: RED, images: [tall, short] }], 1);

	assert.equal(result.width, 5);
	assert.equal(result.height, 4);
	assert.deepEqual(pixel(result, 0, 0), [...RED]);
	assert.deepEqual(pixel(result, 1, 1), [...INK]);
	assert.deepEqual(pixel(result, 1, 2), [...INK]);
	assert.deepEqual(pixel(result, 3, 1), [...RED]);
	assert.deepEqual(pixel(result, 3, 2), [...PAPER]);
});

test("sheet stacks bands and gives every band the widest band's width", () => {
	const BLUE: Rgba = [0, 0, 255, 255];
	const one = rasterize(["#"], 1, colors);
	const result = sheet(
		[
			{ background: BLUE, images: [one, one] },
			{ background: PAPER, images: [one] },
		],
		1,
	);

	assert.equal(result.width, 5);
	assert.equal(result.height, 6);
	assert.deepEqual(pixel(result, 3, 1), [...INK]);
	assert.deepEqual(pixel(result, 4, 4), [...PAPER]);
	assert.deepEqual(pixel(result, 1, 4), [...INK]);
});
