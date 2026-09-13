# Craig Mascot Motion

A self-contained Remotion project for Craig, the Craig Extension mascot: an old sea
captain in two-color pixel art.

## How Craig is drawn

Craig lives on a 56×80 canvas in two colors, ink and paper. Each body part is a text
pixel map in `src/craig/layers/`, where `#` is ink, `.` is paper, and a space is
transparent:

```ts
export const example: Layer = {
	name: "example",
	x: 26,
	y: 10,
	rows: [
		" ## ",
		"#..#",
		"#..#",
		" ## ",
	],
};
```

A pose is a list of layers drawn bottom to top. `src/craig/base.ts` is Craig's neutral
base pose. A new state copies it, then swaps a layer or shifts one by a pixel or two.

- `src/craig/compose.ts` stacks a pose into a grid and rejects malformed layers.
- `src/craig/CraigSprite.tsx` draws a pose as crisp SVG. Pass a whole-number `scale`,
  and `ink` and `paper` colors to recolor him.
- `scripts/export-png.ts` writes PNGs straight from the grid, with no browser.

The approved base is locked by `src/craig/base.snapshot.txt`. If a change to a shared
layer is meant to alter the base, re-export and copy `out/craig-base.txt` over the
snapshot; otherwise the base-pose test fails.

## States

`src/craig/poses.ts` holds the states, each drawn facing screen right: a collapsed
spyglass at his side, a spyglass planted like a cane, a spyglass up to his eye, and a
hand shading his eyes. `sprites` lists every sprite worth exporting: the base, then each
state facing right and left.

`mirrorPose` (`src/craig/mirror.ts`) makes the left-facing versions by flipping each
layer's pixels and position. A layer marked `keepOrientation` moves without flipping, so
the "C" on his hat never reads backwards.

Props are paper with ink outlines and bands, and sleeves carry a paper seam, so nothing
merges into the coat's solid ink. Raised arms split into a sleeve that passes under the
shoulder board and a hand drawn in front of his face.

`npm run export` writes every sprite plus `out/craig-states-overview.png`, which shows
all of them side by side on light and dark.

## Commands

```sh
npm run dev          # Remotion Studio: every sprite still, both loading loops, and the idle loop
npm test             # layers, stacking, mirroring, PNG encoding, pose and animation plans
npm run export       # every sprite at 1x/2x/4x/8x, dark variants, sheets, and text grids
npm run export:idle  # the idle sprite sheet and the CSS the marketing site drops in
npm run lint         # ESLint and tsc
```

## Animations

- `Craig-Telescope-Search` and `Craig-Lookout-Search` are the loading loops, in
  `src/craig/anim/telescope/` and `src/craig/anim/lookout/`. Both start and end on the
  base pose so they loop cleanly, and both hold each drawing for a few frames rather than
  tweening.
- `Craig-Idle-Loop` (`src/craig/idle.ts`) is the marketing-site idle: he breathes, blinks,
  tips his hat, and always settles back into the same resting pose.

Craig fills the canvas from row 0 to row 79, so no pose can shift down as a whole without
falling off it. `nudgeLayers` moves a named group instead, which is how the breath moves
his head and hat together.

`npm run export:idle` writes `out/craig-idle-sheet.png` (one row of equal cells at 1:1),
a dark version, and `out/craig-idle.css`. The CSS animates it with `steps()` and
`image-rendering: pixelated`, so the site can scale it by any whole number and stay crisp,
and it stops the animation under `prefers-reduced-motion`.

## Scenes

Craig fills his own canvas exactly, so anything around him needs a bigger one. `compose`
takes an optional canvas (`compose(pose, { width, height })`), and `src/craig/scene.ts`
uses it: the crow's nest composes on 112×96, with Craig's pose shifted into place by
`shiftPose` and the basket drawn last so it covers him from the hip down. Scene layers
live in `src/craig/layers/scene.ts` and `npm run export` writes them as
`out/craig-crows-nest@{1,2,4,8}x.png`.

Tests and scripts run TypeScript directly through Node's type stripping, so they can't
import `.tsx` files, and type-only imports need `import type`.

## Archive

`archive/miner/` holds the previous miner mascot and its telescope loop. It isn't
built or type-checked. The spyglass timing is kept for Craig's searching state.
