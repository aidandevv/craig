import type { Layer } from "../layer.ts";

/**
 * Craig's bicorne on a filled field, drawn at 16x16 so the extension's 16, 32, 48 and
 * 128 px icons are all whole-number scales of one drawing. Ink field, paper hat, ink "C".
 */
export const hatMark: Layer = {
	name: "hatMark",
	x: 0,
	y: 0,
	rows: [
		"################",
		"################",
		"################",
		"######....######",
		"#####......#####",
		"####........####",
		"####..###...####",
		"###...#......###",
		"##....#.......##",
		"#.....###......#",
		"##............##",
		"####........####",
		"################",
		"################",
		"################",
		"################",
	],
};
