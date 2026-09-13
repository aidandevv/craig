import { CANVAS_WIDTH } from "./layer.ts";
import type { Layer, Pose } from "./layer.ts";

const reverse = (row: string): string => row.split("").reverse().join("");

/** Flips a layer across the canvas's vertical center. Lettering layers move but keep their orientation. */
export const mirrorLayer = (layer: Layer): Layer => {
	const width = layer.rows.length === 0 ? 0 : layer.rows[0].length;
	return {
		...layer,
		rows: layer.keepOrientation ? layer.rows : layer.rows.map(reverse),
		x: CANVAS_WIDTH - layer.x - width,
	};
};

/** Flips a pose left to right, turning a state drawn facing screen right into its screen-left twin. */
export const mirrorPose = (pose: Pose): Pose => pose.map(mirrorLayer);
