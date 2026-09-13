import type { Layer } from "../layer.ts";

export const mouth: Layer = {
	name: "mouth",
	x: 20,
	y: 33,
	rows: [
		"##...#....#...##",
		" #####....##### ",
		"     ######     ",
	],
};

/** A flat, closed line: no lift at the corners, no dip in the middle. */
export const mouthFlat: Layer = {
	name: "mouthFlat",
	x: 20,
	y: 33,
	rows: [
		"  ############  ",
	],
};

/** The neutral mouth's own rows, reversed: the same shallow curve bent the other way. */
export const mouthFrownSlight: Layer = {
	name: "mouthFrownSlight",
	x: 20,
	y: 33,
	rows: [
		"     ######     ",
		" #####....##### ",
		"##...#....#...##",
	],
};

/** The slight frown with its corners pulled down one more row. */
export const mouthFrown: Layer = {
	name: "mouthFrown",
	x: 20,
	y: 33,
	rows: [
		"     ######     ",
		" #####....##### ",
		"##...#....#...##",
		"#.            .#",
	],
};
