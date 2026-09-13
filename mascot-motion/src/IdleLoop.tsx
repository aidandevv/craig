import { AbsoluteFill, useCurrentFrame } from "remotion";
import { CraigSprite } from "./craig/CraigSprite";
import { idleDuration, idleFps, idlePoseAt } from "./craig/idle";

/**
 * The marketing-site idle: Craig breathes, blinks, tips his hat, and always settles back
 * into the same resting pose. Rendered here so the loop can be watched before it becomes
 * a sprite sheet.
 */
export const IdleLoop = () => {
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
			<CraigSprite pose={idlePoseAt(frame)} scale={8} />
		</AbsoluteFill>
	);
};

export const idleLoopComposition = {
	component: IdleLoop,
	durationInFrames: idleDuration,
	fps: idleFps,
	height: 640,
	id: "Craig-Idle-Loop",
	width: 448,
};
