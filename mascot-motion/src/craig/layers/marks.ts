import type { Layer } from "../layer.ts";

/** A big ink "!" cut into a paper plaque, worn on the hat's crown like the badge wears its "C". */
export const exclaimMark: Layer = {
	name: "exclaimMark",
	x: 32,
	y: 5,
	rows: [
		"##########",
		"#........#",
		"#...##...#",
		"#...##...#",
		"#...##...#",
		"#........#",
		"#...##...#",
		"#...##...#",
		"#........#",
		"##########",
	],
};

/** A big ink "?" on the same plaque: a hook curling down from the crown's bar to a separated dot. */
export const questionMark: Layer = {
	name: "questionMark",
	x: 31,
	y: 5,
	rows: [
		"###########",
		"#..######.#",
		"#.......#.#",
		"#.......#.#",
		"#......#..#",
		"#.....#...#",
		"#....#....#",
		"#.........#",
		"#....#....#",
		"###########",
	],
};
