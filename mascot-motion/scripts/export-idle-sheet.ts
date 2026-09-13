import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { compose } from "../src/craig/compose.ts";
import { idleDuration, idleFps, idlePoses } from "../src/craig/idle.ts";
import { CANVAS_HEIGHT, CANVAS_WIDTH } from "../src/craig/layer.ts";
import { encodePng } from "./png.ts";
import { blank, paste, rasterize } from "./raster.ts";
import type { Colors, Rgba } from "./raster.ts";

const BLACK: Rgba = [0, 0, 0, 255];
const WHITE: Rgba = [255, 255, 255, 255];
const CLEAR: Rgba = [0, 0, 0, 0];

/** How many times the site blows the sheet up. Whole numbers only, so pixels stay square. */
const SITE_SCALE = 4;

const outDir = join(import.meta.dirname, "..", "out");
mkdirSync(outDir, { recursive: true });

/**
 * One strip of equal cells at 1:1, so CSS `steps()` lands exactly on each drawing and the
 * site can scale the whole thing up with `image-rendering: pixelated`.
 */
const strip = (colors: Colors) => {
	const sheet = blank(CANVAS_WIDTH * idleDuration, CANVAS_HEIGHT, CLEAR);
	idlePoses.forEach((pose, index) => {
		paste(sheet, rasterize(compose(pose), 1, colors), CANVAS_WIDTH * index, 0);
	});
	return sheet;
};

writeFileSync(join(outDir, "craig-idle-sheet.png"), encodePng(strip({ background: CLEAR, ink: BLACK, paper: WHITE })));
writeFileSync(join(outDir, "craig-idle-sheet-dark.png"), encodePng(strip({ background: CLEAR, ink: WHITE, paper: BLACK })));

const width = CANVAS_WIDTH * SITE_SCALE;
const height = CANVAS_HEIGHT * SITE_SCALE;
const seconds = idleDuration / idleFps;

const css = `/* Craig's idle loop: ${idleDuration} drawings at ${idleFps} fps, ${seconds} seconds, always returning to his resting pose. */
.craig-idle {
	width: ${width}px;
	height: ${height}px;
	background-image: url("craig-idle-sheet.png");
	background-repeat: no-repeat;
	background-size: ${width * idleDuration}px ${height}px;
	image-rendering: pixelated;
	animation: craig-idle ${seconds}s steps(${idleDuration}) infinite;
}

/* On a dark panel, where ink is the light colour. */
.craig-idle--dark {
	background-image: url("craig-idle-sheet-dark.png");
}

@keyframes craig-idle {
	to {
		background-position-x: -${width * idleDuration}px;
	}
}

@media (prefers-reduced-motion: reduce) {
	.craig-idle {
		animation: none;
	}
}
`;

writeFileSync(join(outDir, "craig-idle.css"), css);

console.log(`out/craig-idle-sheet.png and -dark: ${CANVAS_WIDTH * idleDuration}x${CANVAS_HEIGHT}, ${idleDuration} cells`);
console.log(`out/craig-idle.css: ${width}x${height} on the page, ${seconds}s loop`);
