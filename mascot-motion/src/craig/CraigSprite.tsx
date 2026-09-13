import { useMemo } from "react";
import { basePose } from "./base.ts";
import { compose } from "./compose.ts";
import { CANVAS_HEIGHT, CANVAS_WIDTH, INK } from "./layer.ts";
import type { Pose } from "./layer.ts";
import { toRects } from "./rects.ts";

type Props = {
	readonly pose?: Pose;
	/** Whole-number multiple of the 56 x 80 canvas. */
	readonly scale?: number;
	readonly ink?: string;
	readonly paper?: string;
};

/** Craig, the old sea captain, drawn as crisp pixel rectangles on a transparent background. */
export const CraigSprite = ({ ink = "#000000", paper = "#ffffff", pose = basePose, scale = 1 }: Props) => {
	const rects = useMemo(() => toRects(compose(pose)), [pose]);
	if (!Number.isInteger(scale) || scale < 1) {
		throw new Error(`CraigSprite scale must be a positive integer, got ${scale}.`);
	}

	return (
		<svg
			aria-label="Craig, a pixel-art old sea captain"
			height={CANVAS_HEIGHT * scale}
			role="img"
			shapeRendering="crispEdges"
			style={{ display: "block" }}
			viewBox={`0 0 ${CANVAS_WIDTH} ${CANVAS_HEIGHT}`}
			width={CANVAS_WIDTH * scale}
			xmlns="http://www.w3.org/2000/svg"
		>
			{rects.map((rect) => (
				<rect
					fill={rect.paint === INK ? ink : paper}
					height={1}
					key={`${rect.x},${rect.y}`}
					width={rect.width}
					x={rect.x}
					y={rect.y}
				/>
			))}
		</svg>
	);
};
