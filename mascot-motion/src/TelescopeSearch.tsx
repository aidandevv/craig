import { AbsoluteFill, useCurrentFrame } from "remotion";
import { poseAtFrame, TELESCOPE_DURATION, TELESCOPE_FPS } from "./craig/anim/telescope/plan";
import { CraigSprite } from "./craig/CraigSprite";

/** Whole-number multiple of the 56 x 80 art canvas, so every art pixel stays square. */
const SCALE = 8;

/**
 * The loading loop: Craig draws his spyglass, raises it, the lens swells until it fills
 * the frame while his eye hunts across the view, then everything goes back the way it came.
 * Each drawing is held for a few frames rather than tweened.
 */
export const TelescopeSearch = () => {
	const frame = useCurrentFrame();

	return (
		<AbsoluteFill
			style={{
				alignItems: "center",
				backgroundColor: "#ffffff",
				display: "flex",
				justifyContent: "center",
			}}
		>
			<CraigSprite pose={poseAtFrame(frame)} scale={SCALE} />
		</AbsoluteFill>
	);
};

export const telescopeSearchComposition = {
	component: TelescopeSearch,
	durationInFrames: TELESCOPE_DURATION,
	fps: TELESCOPE_FPS,
	height: 80 * SCALE,
	id: "Craig-Telescope-Search",
	width: 56 * SCALE,
};
