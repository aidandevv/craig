import { basePose } from "../../base.ts";
import type { Layer, Pose } from "../../layer.ts";
import { armBehindLeft, handTelescopeRight } from "../../layers/arms.ts";
import { brows, browsSquint } from "../../layers/brows.ts";
import { coat } from "../../layers/coat.ts";
import { epaulettes } from "../../layers/epaulettes.ts";
import { eyes, eyesSquintLeft } from "../../layers/eyes.ts";
import { badge, hat } from "../../layers/hat.ts";
import { head } from "../../layers/head.ts";
import { legs } from "../../layers/legs.ts";
import { mouth } from "../../layers/mouth.ts";
import { mustache } from "../../layers/mustache.ts";
import { nose } from "../../layers/nose.ts";
import { telescopeCollapsed } from "../../layers/telescope.ts";
import { telescopeEyePose } from "../../poses.ts";
import {
	armBehindRight,
	fistDown,
	forearmRight,
	shift,
	shoulderRight,
	sleeveDown,
	sleeveRaised,
	SLEEVE_FULL,
} from "./armStates.ts";
import { craigBrow, craigEye, LENS_CX, LENS_CY, LENS_RADIUS, LENS_RIM } from "./lens.ts";
import type { Glance } from "./lens.ts";
import { discLayer, tubeLayer, vignetteLayer } from "./raster.ts";

/** Everything above the waist that must not move between the standing poses. */
const upperBody = (eyeLayer: Layer, browLayer: Layer): readonly Layer[] => [
	epaulettes,
	head,
	mouth,
	mustache,
	nose,
	eyeLayer,
	browLayer,
];

const crown: readonly Layer[] = [hat, badge];

/** One drawing of the right arm coming out from behind his back with the spyglass. */
type ReachStep = {
	/** Offset of the arm still clasped behind him, or absent once it is out. */
	readonly blob?: readonly [number, number];
	/** Offset of the sleeve, fist and spyglass travelling together. */
	readonly hand?: readonly [number, number];
	readonly shoulder?: true;
};

/**
 * The sleeve, fist and spyglass sit under the coat so they are properly hidden while they
 * are still behind him; at their home position they clear the coat entirely, so the last
 * drawing matches `telescopeSidePose` exactly.
 */
const reachPose = ({ blob, hand, shoulder }: ReachStep): Pose => [
	legs,
	armBehindLeft,
	...(blob ? [shift(armBehindRight, blob[0], blob[1])] : []),
	...(hand ? [shift(telescopeCollapsed, hand[0], hand[1]), shift(forearmRight, hand[0], hand[1])] : []),
	coat,
	...(shoulder ? [shoulderRight] : []),
	...upperBody(eyes, brows),
	...crown,
];

const reachSteps: readonly ReachStep[] = [
	{ blob: [0, 0] },
	{ blob: [1, 0], hand: [-6, -3] },
	{ blob: [2, 1], hand: [-5, -2] },
	{ blob: [3, 1], hand: [-4, -1] },
	{ hand: [-3, -1], shoulder: true },
	{ hand: [-2, 0], shoulder: true },
	{ hand: [-1, 0], shoulder: true },
	{ hand: [0, 0], shoulder: true },
];

/** Base pose through to the collapsed spyglass held at his side. */
export const reachStages: readonly Pose[] = reachSteps.map(reachPose);

/** The hand climbing his side with the spyglass still hanging straight down. */
const climbPose = (sleeveRows: number): Pose => {
	const fistTop = forearmRight.y + sleeveRows;
	return [
		legs,
		armBehindLeft,
		coat,
		...(sleeveRows > 0 ? [sleeveDown(sleeveRows)] : []),
		shoulderRight,
		...upperBody(eyes, brows),
		shift(telescopeCollapsed, 0, fistTop + 1 - telescopeCollapsed.y),
		fistDown(fistTop),
		...crown,
	];
};

/** The spyglass swinging up from vertical to level as the hand reaches his eye. */
type SwingStep = {
	readonly handTop: number;
	readonly handX: number;
	/** 0 is hanging straight down, 1 is level with the horizon. */
	readonly turn: number;
	readonly squint?: true;
};

const swingPose = ({ handTop, handX, squint, turn }: SwingStep): Pose => {
	const angle = (turn * Math.PI) / 2;
	const dirX = Math.sin(angle);
	const dirY = Math.cos(angle);
	const pivotX = handX + 4;
	const pivotY = handTop + 3 - Math.round(6 * turn);
	const objective = 11 - 2 * turn;
	const eyepiece = 3 + 10 * turn;
	const sleeveRows = Math.min(SLEEVE_FULL, 42 - handTop);
	return [
		legs,
		armBehindLeft,
		coat,
		sleeveRaised(sleeveRows),
		...upperBody(squint ? eyesSquintLeft : eyes, squint ? browsSquint : brows),
		tubeLayer("spyglassSwing", {
			ax: pivotX - eyepiece * dirX,
			ay: pivotY - eyepiece * dirY,
			bands: [0.34, 0.62],
			bx: pivotX + objective * dirX,
			by: pivotY + objective * dirY,
			halfWidth: 3.5,
			rim: 1,
		}),
		shift(handTelescopeRight, handX - handTelescopeRight.x, handTop - handTelescopeRight.y),
		...crown,
	];
};

const climbRows: readonly number[] = [8, 5, 2, 0];

const swingSteps: readonly SwingStep[] = [
	{ handTop: 38, handX: 46, turn: 0.15 },
	{ handTop: 33, handX: 45, turn: 0.42 },
	{ handTop: 29, handX: 44, turn: 0.7 },
	{ handTop: 27, handX: 43, squint: true, turn: 0.88 },
];

/** Spyglass at his side through to the spyglass at his eye. The last drawing is `telescopeEyePose`. */
export const raiseStages: readonly Pose[] = [
	...climbRows.map(climbPose),
	...swingSteps.map(swingPose),
	telescopeEyePose,
];

const lerp = (from: number, to: number, t: number): number => from + (to - from) * t;

/** Middle of the objective end while the spyglass is still level with the horizon. */
const OBJECTIVE_X = 51.5;
const OBJECTIVE_Y = 22.5;
/** His eye, which the barrel recedes toward as the far end swings around to face us. */
const EYE_ANCHOR_X = 34;
const EYE_ANCHOR_Y = 22;
/** Radius the objective ring reaches once it is fully end-on, where the growth starts. */
const RING_RADIUS = 7;

const handCentreX = handTelescopeRight.x + handTelescopeRight.rows[0].length / 2;
const handCentreY = handTelescopeRight.y + handTelescopeRight.rows.length / 2;

/**
 * Everything but the objective ring, while he turns the spyglass end-on. The barrel
 * foreshortens toward his eye and his fist rides back along it, so both end up behind the
 * ring rather than beside it. The ring is added last, over the hat, so that once it grows
 * it covers him completely.
 */
const turnedBody = (t: number): readonly Layer[] => {
	const cx = lerp(OBJECTIVE_X, LENS_CX, t);
	const cy = lerp(OBJECTIVE_Y, LENS_CY, t);
	const barrel = lerp(20, 3, t);
	const dx = EYE_ANCHOR_X - cx;
	const dy = EYE_ANCHOR_Y - cy;
	const span = Math.hypot(dx, dy) || 1;
	const backX = cx + (dx / span) * barrel;
	const backY = cy + (dy / span) * barrel;
	return [
		legs,
		armBehindLeft,
		coat,
		sleeveRaised(SLEEVE_FULL),
		...upperBody(eyesSquintLeft, browsSquint),
		tubeLayer("spyglassEndOn", {
			ax: backX,
			ay: backY,
			bands: [0.55],
			bx: cx,
			by: cy,
			halfWidth: 3.5,
			rim: 1,
		}),
		shift(
			handTelescopeRight,
			Math.round(lerp(handCentreX, (backX + cx) / 2, t) - handCentreX),
			Math.round(lerp(handCentreY, (backY + cy) / 2, t) - handCentreY),
		),
		...crown,
	];
};

/** The barrel turning toward the camera until it reads as a ring seen end-on. */
const foreshortenPose = (t: number): Pose => [
	...turnedBody(t),
	discLayer("objectiveRing", {
		cx: lerp(OBJECTIVE_X, LENS_CX, t),
		cy: lerp(OBJECTIVE_Y, LENS_CY, t),
		radius: lerp(3.5, RING_RADIUS, t),
		rim: 2,
	}),
];

/** One drawing of the end-on lens swelling until it fills the frame. */
type GrowStep = {
	readonly radius: number;
	readonly rim: number;
	/** Radius of the hole left in the barrel wall, or absent while it is still off frame. */
	readonly barrel?: number;
	/** Lid opening of the eye once the lens is big enough to hold it. */
	readonly lid?: number;
};

/**
 * The lens grows concentrically about the middle of the frame, where the foreshortened
 * objective already sits, so it swells toward us instead of sliding in from the side.
 */
const growPose = ({ barrel, lid, radius, rim }: GrowStep): Pose => [
	...turnedBody(1),
	...(barrel === undefined ? [] : [vignetteLayer("barrel", LENS_CX, LENS_CY, barrel)]),
	discLayer("lensGlass", { cx: LENS_CX, cy: LENS_CY, radius, rim }),
	...(lid === undefined ? [] : [craigBrow(0), craigEye({ dx: 0, dy: 0, lid })]),
];

const growSteps: readonly GrowStep[] = [
	{ radius: RING_RADIUS, rim: 2 },
	{ barrel: 46, radius: 10, rim: 2 },
	{ barrel: 41, radius: 13, rim: 2 },
	{ barrel: 36, radius: 16, rim: 3 },
	{ barrel: 31, radius: 19, rim: 3 },
	{ barrel: 27, lid: 5, radius: 21.5, rim: 3 },
	{ barrel: LENS_RADIUS, lid: 10, radius: LENS_RADIUS, rim: LENS_RIM },
];

/** Turning the spyglass end-on, before the lens starts to swell. */
export const foreshortenStages: readonly Pose[] = [0.25, 0.5, 0.75].map(foreshortenPose);

/** The end-on lens swelling concentrically until it fills the frame. */
export const growStages: readonly Pose[] = growSteps.map(growPose);

/** The whole way in: barrel end-on, then the lens filling the frame. */
export const arrivalStages: readonly Pose[] = [...foreshortenStages, ...growStages];

/** The settled close-up: nothing but barrel, glass and his eye. */
export const lensPose = (glance: Glance): Pose => [
	vignetteLayer("barrel", LENS_CX, LENS_CY, LENS_RADIUS),
	discLayer("lensGlass", { cx: LENS_CX, cy: LENS_CY, radius: LENS_RADIUS, rim: LENS_RIM }),
	craigBrow(glance.lid <= 0 ? 1 : 0),
	craigEye(glance),
];

/** The base and the side pose, named for the tests and for the reversed half of the loop. */
export const standPose: Pose = basePose;
