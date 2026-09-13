import type { Layer } from "../layer.ts";

export const arms: Layer = {
	name: "arms",
	x: 5,
	y: 42,
	rows: [
		"   #####                              #####   ",
		"  ######                              ######  ",
		" #######                              ####### ",
		" #######                              ####### ",
		" #######                              ####### ",
		" #######                              ####### ",
		" #######                              ####### ",
		" #######                              ####### ",
		" #######                              ####### ",
		"########                              ########",
		"########                              ########",
		"########                              ########",
		"########                              ########",
		"########                              ########",
		" #######                              ####### ",
		"  ######                              ######  ",
		"   #####                              #####   ",
		"    ####                              ####    ",
		"     ###                              ###     ",
	],
};

/** His left arm, still clasped behind his back while the other one works. */
export const armBehindLeft: Layer = {
	name: "armBehindLeft",
	x: 5,
	y: 42,
	rows: [
		"   #####",
		"  ######",
		" #######",
		" #######",
		" #######",
		" #######",
		" #######",
		" #######",
		" #######",
		"########",
		"########",
		"########",
		"########",
		"########",
		" #######",
		"  ######",
		"   #####",
		"    ####",
		"     ###",
	],
};

/** Right arm hanging at his side, fist closed around the collapsed spyglass. */
export const armSideRight: Layer = {
	name: "armSideRight",
	x: 44,
	y: 42,
	rows: [
		"########  ",
		"########  ",
		"########  ",
		"########  ",
		"   ###### ",
		"   #.#### ",
		"   #.#### ",
		"   #.#### ",
		"   #.#### ",
		"   #....# ",
		"   #.#### ",
		"   #....# ",
		"   #.#### ",
		"   #....# ",
		"   ###### ",
		"  ###....#",
		"  #......#",
		"  #...####",
		"  #......#",
		"  #...####",
		"   ###### ",
	],
};

/** Right arm reaching down, hand resting on the planted spyglass. */
export const armCaneRight: Layer = {
	name: "armCaneRight",
	x: 44,
	y: 42,
	rows: [
		"########    ",
		"########    ",
		"########    ",
		"########    ",
		"  ##.####   ",
		"  ##.#...   ",
		"   ##.####  ",
		"   ##.####  ",
		"    ####### ",
		"     ###### ",
		"    #......#",
		"    #.#.#..#",
		"    #.#.#..#",
		"     ###### ",
	],
};

/** The raised sleeve for the spyglass pose; it passes under the shoulder board. */
export const sleeveTelescopeRight: Layer = {
	name: "sleeveTelescopeRight",
	x: 48,
	y: 30,
	rows: [
		"#.####",
		"#.####",
		"#....#",
		"#.####",
		"#....#",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
	],
};

/** His fist wrapped around the spyglass barrel, in front of his face. */
export const handTelescopeRight: Layer = {
	name: "handTelescopeRight",
	x: 41,
	y: 24,
	rows: [
		"   ####  ",
		"  ###### ",
		" #.#.#..#",
		" #.#.#..#",
		" #.#.#..#",
		" #.#.#..#",
		" #.#.#..#",
		"  ###### ",
	],
};

/** The raised sleeve for the lookout pose; it passes under the shoulder board. */
export const sleeveShadeRight: Layer = {
	name: "sleeveShadeRight",
	x: 48,
	y: 18,
	rows: [
		"#.####",
		"#.####",
		"#....#",
		"#.####",
		"#....#",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
	],
};

/** His hand held flat over his brow like a visor, fingers across his face. */
export const handShadeRight: Layer = {
	name: "handShadeRight",
	x: 33,
	y: 15,
	rows: [
		"            ########",
		"      ######.......#",
		"######..#...##.....#",
		"#..#..######  #...# ",
		"######        ##### ",
	],
};

