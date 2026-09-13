import type { CSSProperties } from "react";

type Props = {
	readonly size?: number;
};

/** Craig's canonical, prop-free silhouette. */
export const CraigSprite = ({ size = 64 }: Props) => {
	const style: CSSProperties = { background: "#fff", display: "block", imageRendering: "pixelated" };

	return (
		<svg
			aria-label="Craig, a monochrome pixel-art investigator"
			fill="#000"
			height={size}
			role="img"
			shapeRendering="crispEdges"
			style={style}
			viewBox="0 0 64 64"
			width={size}
			xmlns="http://www.w3.org/2000/svg"
		>
			<rect fill="#fff" height="64" width="64" />
			<path d="M9 56h46v2H9z" />
			{/* Black outline first; Craig stays mostly white. */}
			<rect x="21" y="14" width="18" height="3" />
			<rect x="19" y="17" width="22" height="5" />
			<rect x="17" y="22" width="26" height="5" />
			<rect x="15" y="27" width="31" height="3" />
			<rect x="19" y="30" width="22" height="10" />
			<rect x="20" y="39" width="22" height="11" />
			<rect x="18" y="42" width="6" height="8" />
			<rect x="36" y="40" width="7" height="7" />
			<rect x="20" y="49" width="10" height="8" />
			<rect x="32" y="49" width="10" height="8" />
			{/* White fill keeps the body from becoming a detailed illustration. */}
			<rect fill="#fff" x="22" y="17" width="16" height="3" />
			<rect fill="#fff" x="20" y="20" width="20" height="4" />
			<rect fill="#fff" x="19" y="24" width="22" height="2" />
			<rect fill="#fff" x="21" y="31" width="18" height="7" />
			<rect fill="#fff" x="22" y="40" width="18" height="8" />
			<rect fill="#fff" x="20" y="43" width="2" height="5" />
			<rect fill="#fff" x="38" y="41" width="3" height="4" />
			<rect fill="#fff" x="22" y="50" width="6" height="5" />
			<rect fill="#fff" x="34" y="50" width="6" height="5" />
			{/* The two-pixel, watchful visor. */}
			<rect x="33" y="33" width="4" height="2" />
		</svg>
	);
};
