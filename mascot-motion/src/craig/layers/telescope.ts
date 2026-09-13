import type { Layer } from "../layer.ts";

/** The spyglass collapsed, gripped in his fist. */
export const telescopeCollapsed: Layer = {
	name: "telescopeCollapsed",
	x: 47,
	y: 58,
	rows: [
		" ##### ",
		" #...# ",
		" #...# ",
		" ##### ",
		" #...# ",
		" #...# ",
		" #...# ",
		" ##### ",
		" #...# ",
		" #...# ",
		"#######",
		"#.....#",
		"#.....#",
		"#######",
	],
};

/** The spyglass drawn out to full length, planted on the ground. */
export const telescopeExtended: Layer = {
	name: "telescopeExtended",
	x: 49,
	y: 55,
	rows: [
		"  ###  ",
		"  #.#  ",
		"  #.#  ",
		"  #.#  ",
		"  #.#  ",
		"  ###  ",
		" ##### ",
		" #...# ",
		" #...# ",
		" #...# ",
		" #...# ",
		" #...# ",
		" #...# ",
		" #...# ",
		" ##### ",
		"#######",
		"#.....#",
		"#.....#",
		"#.....#",
		"#.....#",
		"#.....#",
		"#.....#",
		"#.....#",
		"#.....#",
		"#######",
	],
};

/** The spyglass held to his eye, pointing toward the horizon. */
export const telescopeRaised: Layer = {
	name: "telescopeRaised",
	x: 33,
	y: 19,
	rows: [
		"              #########",
		"      #########....#..#",
		"#######......##....#..#",
		"#....##......##....#..#",
		"#######......##....#..#",
		"      #########....#..#",
		"              #########",
	],
};
