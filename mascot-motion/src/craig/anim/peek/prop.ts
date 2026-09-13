import type { Layer } from "../../layer.ts";
import { telescopeCollapsed, telescopeRaised } from "../../layers/telescope.ts";

/** The stowed spyglass with only its top rows drawn, so it slides into view out of his fist. */
export const stowedDrawn = (rows: number): Layer => ({
	...telescopeCollapsed,
	name: `peekStowed${rows}`,
	rows: telescopeCollapsed.rows.slice(0, rows),
});

/** The collapsed spyglass carried upright, its top at `top`. */
export const upright = (top: number): Layer => ({ ...telescopeCollapsed, name: `peekUpright${top}`, y: top });

/** The sighting spyglass carried `drop` rows below the height it sights from. */
export const raised = (drop: number): Layer => ({
	...telescopeRaised,
	name: `peekRaised${drop}`,
	y: telescopeRaised.y + drop,
});

/** The spyglass caught halfway round, swinging up from his side toward his eye. */
export const glassTipping: Layer = {
	name: "peekGlassTipping",
	rows: [
		"####       ",
		"#..##      ",
		"##..##     ",
		" ##..##    ",
		"  ##..##   ",
		"   ##..##  ",
		"    ##..## ",
		"     ##..##",
		"      ##..#",
		"       ####",
	],
	x: 44,
	y: 46,
};

/** The spyglass swung part way toward the viewer, its barrel foreshortened. */
export const glassShort: Layer = {
	name: "peekGlassShort",
	rows: [
		"      #########",
		"      #....#..#",
		"#######....#..#",
		"#....##....#..#",
		"#######....#..#",
		"      #....#..#",
		"      #########",
	],
	x: 33,
	y: 19,
};

/** The spyglass pointed straight at the viewer: all barrel gone, just the rings of the lens. */
export const glassLens: Layer = {
	name: "peekGlassLens",
	rows: [
		"   ######   ",
		" ##......## ",
		" #........# ",
		"#...####...#",
		"#..######..#",
		"#..######..#",
		"#...####...#",
		" #........# ",
		" ##......## ",
		"   ######   ",
	],
	x: 30,
	y: 17,
};
