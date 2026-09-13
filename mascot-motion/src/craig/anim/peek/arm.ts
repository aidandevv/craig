import type { Layer } from "../../layer.ts";
import { armBehindLeft, handTelescopeRight, sleeveTelescopeRight } from "../../layers/arms.ts";
import { mirrorLayer } from "../../mirror.ts";

/** The resting right arm: the base pose's, mirrored from the one clasped behind his back. */
export const restArm: Layer = mirrorLayer(armBehindLeft);

/** His fist, moved to close around the spyglass wherever it has got to. */
export const fist = (x: number, y: number): Layer => ({ ...handTelescopeRight, name: `peekFist${x}x${y}`, x, y });

/** The raised sleeve cut off at `top`, so the arm grows out of the shoulder board as it rises. */
export const raisedSleeve = (top: number): Layer => {
	const dropped = Math.max(0, top - sleeveTelescopeRight.y);
	return {
		...sleeveTelescopeRight,
		name: `peekSleeve${top}`,
		rows: sleeveTelescopeRight.rows.slice(dropped),
		y: sleeveTelescopeRight.y + dropped,
	};
};

/** The sleeve hanging at his side, without the fist that the side-arm layer carries with it. */
const hangingRows: readonly string[] = [
	"######",
	"#.####",
	"#.####",
	"#.####",
	"#.####",
	"#....#",
	"#.####",
	"#....#",
	"#.####",
	"#....#",
	"######",
];

const HANGING_TOP = 46;

/** The hanging sleeve ending at `bottom`, so it shortens as his elbow folds. */
export const hangingSleeve = (bottom: number): Layer => ({
	name: `peekHanging${bottom}`,
	rows: hangingRows.slice(0, Math.max(1, bottom - HANGING_TOP + 1)),
	x: 47,
	y: HANGING_TOP,
});
