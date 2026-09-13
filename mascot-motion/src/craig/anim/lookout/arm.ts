import type { Layer } from "../../layer.ts";
import { armBehindLeft, handShadeRight, handTelescopeRight, sleeveShadeRight } from "../../layers/arms.ts";
import { mirrorLayer } from "../../mirror.ts";

/**
 * One drawing of Craig's right arm on its way up to his brow.
 *
 * The sleeve sits where the resting arm sits: behind the coat, under the shoulder board.
 * The hand sits in front of his face, the way the shading hand does.
 */
export type ArmDrawing = {
	readonly name: string;
	readonly sleeve: Layer;
	readonly hand?: Layer;
};

/** His closed hand, swung out from behind his back and riding up his side. */
const fist = (y: number): Layer => ({ ...handTelescopeRight, name: `lookoutFist${y}`, x: 47, y });

/** The resting sleeve: the base pose's right arm, mirrored from the one clasped behind his back. */
const restSleeve: Layer = mirrorLayer(armBehindLeft);

/** The lookout sleeve cut off at `top`, so the arm grows out of the shoulder board as it rises. */
const raisedSleeve = (top: number): Layer => {
	const dropped = Math.max(0, top - sleeveShadeRight.y);
	return {
		...sleeveShadeRight,
		name: `lookoutSleeve${top}`,
		rows: sleeveShadeRight.rows.slice(dropped),
		y: sleeveShadeRight.y + dropped,
	};
};

/** The flat hand nudged off its resting place: on its way in, or tipped while he searches. */
export const shadeHand = (dx: number, dy: number): Layer => ({
	...handShadeRight,
	name: `lookoutShadeHand${dx}x${dy}`,
	x: handShadeRight.x + dx,
	y: handShadeRight.y + dy,
});

/** The sleeve swung forward off his back, elbow bent, the cuff coming round his hip. */
const sleeveSwingLow: Layer = {
	name: "lookoutSleeveSwingLow",
	rows: [
		"#####     ",
		"######    ",
		"#######   ",
		"#.#####   ",
		"#.#####   ",
		"#.#####   ",
		"#.######  ",
		" #.###### ",
		" #.###### ",
		" #.#######",
		"  #.######",
		"  #.######",
		"  ########",
	],
	x: 43,
	y: 42,
};

/** The sleeve out at shoulder height, the elbow lifting clear of the coat. */
const sleeveSwingOut: Layer = {
	name: "lookoutSleeveSwingOut",
	rows: [
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		"#.####",
		" #####",
		"  ### ",
	],
	x: 49,
	y: 44,
};

/**
 * The arm's climb, drawing by drawing: the hand out from behind his back, up his side,
 * past his cheek, then flat across his brow. The last drawing is the lookout arm itself.
 */
export const armLift: readonly ArmDrawing[] = [
	{ name: "rest", sleeve: restSleeve },
	{ hand: fist(55), name: "handOut", sleeve: restSleeve },
	{ hand: fist(48), name: "swingLow", sleeve: sleeveSwingLow },
	{ hand: fist(41), name: "swingOut", sleeve: sleeveSwingOut },
	{ hand: fist(34), name: "pastShoulder", sleeve: raisedSleeve(38) },
	{ hand: fist(28), name: "pastCheek", sleeve: raisedSleeve(32) },
	{ hand: fist(23), name: "atTemple", sleeve: raisedSleeve(27) },
	{ hand: shadeHand(3, 3), name: "flattening", sleeve: raisedSleeve(23) },
	{ hand: shadeHand(2, 2), name: "arriving", sleeve: raisedSleeve(22) },
	{ hand: handShadeRight, name: "shading", sleeve: sleeveShadeRight },
];

/** The lookout arm with the hand tipped up a pixel, the way it shifts mid-search. */
export const armTipped: ArmDrawing = {
	hand: shadeHand(0, -1),
	name: "tipped",
	sleeve: sleeveShadeRight,
};
