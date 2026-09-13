import type { Layer, Pose } from "../../layer.ts";
import { armBehindLeft } from "../../layers/arms.ts";
import { coat } from "../../layers/coat.ts";
import { epaulettes } from "../../layers/epaulettes.ts";
import { badge, hat } from "../../layers/hat.ts";
import { head } from "../../layers/head.ts";
import { legs } from "../../layers/legs.ts";
import { mouth } from "../../layers/mouth.ts";
import { mustache } from "../../layers/mustache.ts";
import { nose } from "../../layers/nose.ts";
import type { ArmDrawing } from "./arm.ts";

/**
 * A standing lookout frame. Hat, head, coat and legs are the same layers in every drawing,
 * so only the arm, the eyes and the brows ever move.
 */
export const lookoutPose = (arm: ArmDrawing, eyesLayer: Layer, browsLayer: Layer): Pose => {
	const pose: Layer[] = [
		legs,
		armBehindLeft,
		arm.sleeve,
		coat,
		epaulettes,
		head,
		mouth,
		mustache,
		nose,
		eyesLayer,
		browsLayer,
	];
	if (arm.hand) {
		pose.push(arm.hand);
	}
	pose.push(hat, badge);
	return pose;
};
