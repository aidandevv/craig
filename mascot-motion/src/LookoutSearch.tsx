import { AbsoluteFill, useCurrentFrame } from "remotion";
import { lookoutDuration, lookoutFps, lookoutFrame } from "./craig/anim/lookout/frames";
import { CraigSprite } from "./craig/CraigSprite";

/**
 * The lookout loop: Craig stands, brings a hand up flat over his brow, searches the
 * horizon, then lowers it back to exactly the pose he started in.
 */
export const LookoutSearch = () => {
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
			<CraigSprite pose={lookoutFrame(frame)} scale={8} />
		</AbsoluteFill>
	);
};

export const lookoutSearchComposition = {
	component: LookoutSearch,
	durationInFrames: lookoutDuration,
	fps: lookoutFps,
	height: 640,
	id: "Craig-Lookout-Search",
	width: 448,
};
