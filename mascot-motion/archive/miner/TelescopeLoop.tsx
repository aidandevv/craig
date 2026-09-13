import { AbsoluteFill, Easing, interpolate, useCurrentFrame } from "remotion";
import { CraigSprite } from "./CraigSprite";

const ease = Easing.bezier(0.22, 0.8, 0.24, 1);

const Telescope = ({ frame }: { readonly frame: number }) => (
	<div
		style={{
			left: interpolate(frame, [24, 52, 104, 204, 224, 238], [80, 80, 51, 51, 80, 80], {
				easing: ease, extrapolateLeft: "clamp", extrapolateRight: "clamp",
			}),
			/* The lens covers the prop before it fades; no visible layer handoff. */
			opacity: interpolate(frame, [18, 24, 126, 136, 186, 198, 232, 238], [0, 1, 1, 0, 0, 1, 1, 0], {
				extrapolateLeft: "clamp", extrapolateRight: "clamp",
			}),
			position: "absolute",
			rotate: `${interpolate(frame, [52, 104, 204, 224], [0, -90, -90, 0], {
				easing: ease, extrapolateLeft: "clamp", extrapolateRight: "clamp",
			})}deg`,
			top: interpolate(frame, [24, 52, 104, 204, 224, 238], [116, 46, 38, 38, 46, 116], {
				easing: ease, extrapolateLeft: "clamp", extrapolateRight: "clamp",
			}),
			transformOrigin: "18px 62px",
		}}
	>
		<svg aria-label="Craig's spyglass" height="80" shapeRendering="crispEdges" viewBox="0 0 36 80" width="36">
			{/* Rim, nested body, two bands, and a capped handle. */}
			<rect fill="#000" height="16" width="28" x="4" y="2" />
			<rect fill="#fff" height="10" width="20" x="8" y="5" />
			<rect fill="#000" height="48" width="20" x="8" y="15" />
			<rect fill="#fff" height="42" width="12" x="12" y="18" />
			<rect fill="#000" height="4" width="24" x="6" y="23" />
			<rect fill="#000" height="4" width="24" x="6" y="45" />
			<rect fill="#000" height="17" width="24" x="6" y="60" />
			<rect fill="#fff" height="11" width="16" x="10" y="63" />
			<rect fill="#000" height="6" width="12" x="12" y="74" />
		</svg>
	</div>
);

const TelescopeLens = ({ blink, frame }: { readonly blink: number; readonly frame: number }) => {
	const diameter = interpolate(frame, [96, 136, 176, 214], [10, 188, 188, 10], {
		easing: ease, extrapolateLeft: "clamp", extrapolateRight: "clamp",
	});
	const centerX = interpolate(frame, [96, 136, 176, 214], [76, 64, 64, 76], {
		easing: ease, extrapolateLeft: "clamp", extrapolateRight: "clamp",
	});
	const centerY = interpolate(frame, [96, 136, 176, 214], [58, 64, 64, 58], {
		easing: ease, extrapolateLeft: "clamp", extrapolateRight: "clamp",
	});

	return (
		<svg
			height={diameter}
			shapeRendering="crispEdges"
			style={{ left: centerX - diameter / 2, position: "absolute", top: centerY - diameter / 2 }}
			viewBox="0 0 64 64"
			width={diameter}
		>
			{/* Pixel-stepped circular lens. */}
			<path d="M16 0h32v4h8v8h4v40h-4v8h-8v4H16v-4H8v-8H4V12h4V4h8z" fill="#000" />
			<path d="M17 5h30v4h7v7h4v32h-4v7h-7v4H17v-4h-7v-7H6V16h4V9h7z" fill="#fff" />
			{/* Eyebrow, socket, sclera, stepped iris, pupil, and glint. */}
			<rect fill="#000" height="4" width="34" x="15" y="20" />
			<rect fill="#000" height="18" width="38" x="13" y="25" />
			<rect fill="#fff" height="12" width="32" x="16" y="28" />
			<rect fill="#000" height="2" width="8" x="28" y="28" />
			<rect fill="#000" height="8" width="12" x="26" y="30" />
			<rect fill="#000" height="2" width="8" x="28" y="38" />
			<rect fill="#fff" height="6" width="6" x="29" y="31" />
			<rect fill="#000" height="4" width="4" x="30" y="32" />
			<rect fill="#fff" height="2" width="2" x="27" y="29" />
			{/* Smooth eyelid closure over the detailed eye. */}
			<rect fill="#000" height={blink * 6} width="32" x="16" y="28" />
			<rect fill="#000" height={blink * 6} width="32" x="16" y={40 - blink * 6} />
			{blink > 0.85 ? <rect fill="#fff" height="1" width="24" x="20" y="33" /> : null}
		</svg>
	);
};

/** A detailed ten-second spyglass loop at 24 fps. */
export const CraigTelescopeLoop = () => {
	const frame = useCurrentFrame();
	const blink = interpolate(frame, [146, 154, 162, 170], [0, 1, 1, 0], {
		easing: Easing.bezier(0.3, 0, 0.7, 1), extrapolateLeft: "clamp", extrapolateRight: "clamp",
	});
	const lensVisible = frame >= 96 && frame <= 214;

	return (
		<AbsoluteFill style={{ alignItems: "center", backgroundColor: "#fff", display: "flex", justifyContent: "center", overflow: "hidden" }}>
			<CraigSprite size={128} />
			<Telescope frame={frame} />
			{lensVisible ? <TelescopeLens blink={blink} frame={frame} /> : null}
		</AbsoluteFill>
	);
};
