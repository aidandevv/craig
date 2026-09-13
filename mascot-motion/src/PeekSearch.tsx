import { AbsoluteFill, useCurrentFrame } from "remotion";
import { peekDuration, peekFps, peekFrame } from "./craig/anim/peek/frames";
import { CraigSprite } from "./craig/CraigSprite";

/** Whole-number multiple of the 56 x 80 art canvas, so every art pixel stays square. */
const SCALE = 8;

/**
 * The peek loop: Craig draws his spyglass, raises it to his eye, looks one way, round past
 * the viewer, then the other way, and finally stows it and stands as he began.
 */
export const PeekSearch = () => {
	const frame = useCurrentFrame();

	return (
		<AbsoluteFill
			style={{
				alignItems: "center",
				backgroundColor: "#f8faf7",
				display: "flex",
				justifyContent: "center",
			}}
		>
			<CraigSprite pose={peekFrame(frame)} scale={SCALE} />
		</AbsoluteFill>
	);
};

export const peekSearchComposition = {
	component: PeekSearch,
	durationInFrames: peekDuration,
	fps: peekFps,
	height: 80 * SCALE,
	id: "Craig-Peek-Search",
	width: 56 * SCALE,
};
