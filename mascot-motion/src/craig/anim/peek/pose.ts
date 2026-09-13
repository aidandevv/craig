import type { Layer, Pose } from "../../layer.ts";
import { armBehindLeft } from "../../layers/arms.ts";
import { brows } from "../../layers/brows.ts";
import { coat } from "../../layers/coat.ts";
import { epaulettes } from "../../layers/epaulettes.ts";
import { eyes } from "../../layers/eyes.ts";
import { badge, hat } from "../../layers/hat.ts";
import { head } from "../../layers/head.ts";
import { legs } from "../../layers/legs.ts";
import { mouth } from "../../layers/mouth.ts";
import { mustache } from "../../layers/mustache.ts";
import { nose } from "../../layers/nose.ts";

/**
 * One drawing of the peek loop. The slots follow the order the spyglass poses already use:
 * the stowed glass sits behind the arm at his side, the raised glass and his fist sit over his face.
 */
export type PeekDrawing = {
	readonly name: string;
	/** The collapsed spyglass down at his side, behind the arm. */
	readonly stowed?: Layer;
	/** The arm or sleeve, under the shoulder board. */
	readonly sleeve?: Layer;
	/** The spyglass raised in front of his face. */
	readonly glass?: Layer;
	/** His fist, closed over whatever he is holding. */
	readonly hand?: Layer;
	readonly eyes?: Layer;
	readonly brows?: Layer;
};

/**
 * Stacks a peek drawing. Hat, head, coat and legs are the same layers in every drawing,
 * so only the arm, the spyglass, the eyes and the brows ever move.
 */
export const peekPose = (drawing: PeekDrawing): Pose => {
	const pose: Layer[] = [legs, armBehindLeft, coat];
	if (drawing.stowed) {
		pose.push(drawing.stowed);
	}
	if (drawing.sleeve) {
		pose.push(drawing.sleeve);
	}
	pose.push(epaulettes, head, mouth, mustache, nose, drawing.eyes ?? eyes, drawing.brows ?? brows);
	if (drawing.glass) {
		pose.push(drawing.glass);
	}
	if (drawing.hand) {
		pose.push(drawing.hand);
	}
	pose.push(hat, badge);
	return pose;
};
