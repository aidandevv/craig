import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { compose, gridToText } from "../src/craig/compose.ts";
import { sprites } from "../src/craig/poses.ts";
import { crowsNestScene, CROWS_NEST_CANVAS, GULL_CANVAS, gullLevelPose, gullUpPose, hatMarkPose, MARK_CANVAS, waveTilePose, WAVE_TILE_CANVAS } from "../src/craig/scene.ts";
import { encodePng } from "./png.ts";
import { rasterize, sheet } from "./raster.ts";
import type { Colors, Rgba } from "./raster.ts";

const BLACK: Rgba = [0, 0, 0, 255];
const WHITE: Rgba = [255, 255, 255, 255];
const CLEAR: Rgba = [0, 0, 0, 0];
const SCALES = [1, 2, 4, 8];

const light: Colors = { background: CLEAR, ink: BLACK, paper: WHITE };
const dark: Colors = { background: CLEAR, ink: WHITE, paper: BLACK };

const outDir = join(import.meta.dirname, "..", "out");
mkdirSync(outDir, { recursive: true });

const write = (name: string, contents: Buffer | string): void => {
	writeFileSync(join(outDir, name), contents);
};

for (const [name, pose] of sprites) {
	const grid = compose(pose);
	for (const scale of SCALES) {
		write(`craig-${name}@${scale}x.png`, encodePng(rasterize(grid, scale, light)));
		write(`craig-${name}-dark@${scale}x.png`, encodePng(rasterize(grid, scale, dark)));
	}
	write(`craig-${name}.txt`, gridToText(grid));
	write(
		`craig-${name}-sheet.png`,
		encodePng(
			sheet(
				[
					{ background: WHITE, images: SCALES.map((scale) => rasterize(grid, scale, { ...light, background: WHITE })) },
					{ background: BLACK, images: SCALES.map((scale) => rasterize(grid, scale, { ...dark, background: BLACK })) },
				],
				16,
			),
		),
	);
	console.log(`out/craig-${name}@{1,2,4,8}x.png, dark variants, sheet, and grid`);
}

/** One image with every sprite side by side, for reviewing the whole set at a glance. */
write(
	"craig-states-overview.png",
	encodePng(
		sheet(
			[
				{ background: WHITE, images: sprites.map(([, pose]) => rasterize(compose(pose), 2, { ...light, background: WHITE })) },
				{ background: BLACK, images: sprites.map(([, pose]) => rasterize(compose(pose), 2, { ...dark, background: BLACK })) },
			],
			16,
		),
	),
);
console.log("out/craig-states-overview.png");

/** Scenes compose onto their own, larger canvas, with Craig standing somewhere inside it. */
const sceneGrid = compose(crowsNestScene, CROWS_NEST_CANVAS);
for (const scale of SCALES) {
	write(`craig-crows-nest@${scale}x.png`, encodePng(rasterize(sceneGrid, scale, light)));
	write(`craig-crows-nest-dark@${scale}x.png`, encodePng(rasterize(sceneGrid, scale, dark)));
}
write("craig-crows-nest.txt", gridToText(sceneGrid));
console.log("out/craig-crows-nest@{1,2,4,8}x.png, dark variants, and grid");

/** Site assets: the ocean tile and gull frames, drawn once at 1:1 for the marketing page. */
write("craig-wave-tile.png", encodePng(rasterize(compose(waveTilePose, WAVE_TILE_CANVAS), 1, light)));
write("craig-gull-up.png", encodePng(rasterize(compose(gullUpPose, GULL_CANVAS), 1, light)));
write("craig-gull-level.png", encodePng(rasterize(compose(gullLevelPose, GULL_CANVAS), 1, light)));
console.log("out/craig-wave-tile.png, craig-gull-up.png, and the gull frames");

/** The extension icon, at every size Chrome asks for. Each one is a whole-number scale of the 16x16 mark. */
const markGrid = compose(hatMarkPose, MARK_CANVAS);
for (const size of [16, 32, 48, 128]) {
	write(`craig-icon-${size}.png`, encodePng(rasterize(markGrid, size / MARK_CANVAS.width, light)));
}
console.log("out/craig-icon-{16,32,48,128}.png");
