import type { Layer } from "../layer.ts";

export const brows: Layer = {
	name: "brows",
	x: 14,
	y: 16,
	rows: [
		"    ## ##          ## ##    ",
		"  ##..#..##      ##..#..##  ",
		"##.........#    #.........##",
		"#.....######    ######.....#",
		" #####                ##### ",
	],
};

/** The squinting brow drops a row; the other keeps its place. */
export const browsSquint: Layer = {
	name: "browsSquint",
	x: 14,
	y: 16,
	rows: [
		"                   ## ##    ",
		"    ## ##        ##..#..##  ",
		"  ##..#..##     #.........##",
		"##.........#    ######.....#",
		"#.....######          ##### ",
		" #####                      ",
	],
};

/** Both brows angled down toward the nose: a thick diagonal stroke on each side, high at the temple, low at the bridge. */
export const browsAngry: Layer = {
	name: "browsAngry",
	x: 14,
	y: 16,
	rows: [
		"##                        ##",
		" ###                    ### ",
		"  ####                ####  ",
		"   #####            #####   ",
		"    ######        ######    ",
	],
};
