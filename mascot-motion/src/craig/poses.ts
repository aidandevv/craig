import { basePose } from "./base.ts";
import type { Pose } from "./layer.ts";
import {
	armBehindLeft,
	armCaneRight,
	arms,
	armSideRight,
	handShadeRight,
	handTelescopeRight,
	sleeveShadeRight,
	sleeveTelescopeRight,
} from "./layers/arms.ts";
import { brows, browsAngry, browsSquint } from "./layers/brows.ts";
import { coat } from "./layers/coat.ts";
import { epaulettes } from "./layers/epaulettes.ts";
import { eyes, eyesLookRight, eyesSquintLeft } from "./layers/eyes.ts";
import { badge, hat } from "./layers/hat.ts";
import { head } from "./layers/head.ts";
import { legs } from "./layers/legs.ts";
import { exclaimMark, questionMark } from "./layers/marks.ts";
import { mouth, mouthFlat, mouthFrown, mouthFrownSlight } from "./layers/mouth.ts";
import { mustache } from "./layers/mustache.ts";
import { nose } from "./layers/nose.ts";
import { telescopeCollapsed, telescopeExtended, telescopeRaised } from "./layers/telescope.ts";
import { profileArmShade, profileBase } from "./layers/profile.ts";
import { craigTurned, handTurned, telescopeTurned } from "./layers/turned.ts";
import { mirrorPose } from "./mirror.ts";

/** A collapsed spyglass held at his side, ready to be raised. */
export const telescopeSidePose: Pose = [
	legs,
	armBehindLeft,
	coat,
	telescopeCollapsed,
	armSideRight,
	epaulettes,
	head,
	mouth,
	mustache,
	nose,
	eyes,
	brows,
	hat,
	badge,
];

/** The spyglass drawn out and planted on the ground, his hand resting on top. */
export const telescopeCanePose: Pose = [
	legs,
	armBehindLeft,
	coat,
	telescopeExtended,
	armCaneRight,
	epaulettes,
	head,
	mouth,
	mustache,
	nose,
	eyes,
	brows,
	hat,
	badge,
];

/** Sighting down the spyglass: the far eye shut, the spyglass across his face. */
export const telescopeEyePose: Pose = [
	legs,
	armBehindLeft,
	coat,
	sleeveTelescopeRight,
	epaulettes,
	head,
	mouth,
	mustache,
	nose,
	eyesSquintLeft,
	browsSquint,
	telescopeRaised,
	handTelescopeRight,
	hat,
	badge,
];

/** Hand flat over his brow, peering out over the horizon. */
export const shadeEyesPose: Pose = [
	legs,
	armBehindLeft,
	coat,
	sleeveShadeRight,
	epaulettes,
	head,
	mouth,
	mustache,
	nose,
	eyesLookRight,
	brows,
	handShadeRight,
	hat,
	badge,
];

/** All facing the viewer head-on: the badge's risk-band states, least to most concerning, plus one non-band state for incomplete evidence. */

/** Low concern: pleased and at ease. The same resting pose as the base — nothing here needs a reaction. */
export const lowConcernPose: Pose = basePose;

/** Caution: a flat, unreadable expression. */
export const cautionPose: Pose = [legs, arms, coat, epaulettes, head, mouthFlat, mustache, nose, eyes, brows, hat, badge];

/** Elevated: one brow lower than the other, mouth turned down slightly. */
export const elevatedConcernPose: Pose = [
	legs,
	arms,
	coat,
	epaulettes,
	head,
	mouthFrownSlight,
	mustache,
	nose,
	eyes,
	browsSquint,
	hat,
	badge,
];

/** High concern: both brows down in a real frown. */
export const highConcernPose: Pose = [legs, arms, coat, epaulettes, head, mouthFrown, mustache, nose, eyes, browsAngry, hat, badge];

/** Hard flag: the same angry frown as high concern, with a mark pinned to the hat. */
export const hardFlagPose: Pose = [...highConcernPose, exclaimMark];

/** Incomplete: the same flat, unreadable face as caution, with a mark on the hat asking the question outright. */
export const incompletePose: Pose = [...cautionPose, questionMark];

/** A test drawing: Craig turned toward screen right, so the spyglass points away from his face. */
export const turnedTelescopePose: Pose = [craigTurned, telescopeTurned, handTurned];

/** Craig side-on, facing screen right, at rest. */
export const profileBasePose: Pose = [profileBase];

/** Side-on with his hand flat over his brow, peering out over the horizon. */
export const profileShadePose: Pose = [profileBase, profileArmShade];

/** The states that face a direction, drawn facing screen right. */
const directional: readonly (readonly [string, Pose])[] = [
	["telescope-side", telescopeSidePose],
	["telescope-cane", telescopeCanePose],
	["telescope-eye", telescopeEyePose],
	["shade-eyes", shadeEyesPose],
];

/** Every sprite worth exporting: the symmetric base, then each state facing right and left. */
export const sprites: readonly (readonly [string, Pose])[] = [
	["base", basePose],
	...directional.map(([name, pose]) => [`${name}-right`, pose] as const),
	...directional.map(([name, pose]) => [`${name}-left`, mirrorPose(pose)] as const),
	["turned-telescope", turnedTelescopePose],
	["low-concern", lowConcernPose],
	["caution", cautionPose],
	["elevated-concern", elevatedConcernPose],
	["high-concern", highConcernPose],
	["hard-flag", hardFlagPose],
	["incomplete", incompletePose],
];
