/** Craig's canvas size, in art pixels. */
export const CANVAS_WIDTH = 56;
export const CANVAS_HEIGHT = 80;

/** Layer characters: ink, paper, and transparent. */
export const INK = "#";
export const PAPER = ".";
export const CLEAR = " ";

export type Paint = typeof INK | typeof PAPER;

/** One body part: a text pixel map placed on the canvas. */
export type Layer = {
	readonly name: string;
	/** Canvas column of the layer's left edge. */
	readonly x: number;
	/** Canvas row of the layer's top edge. */
	readonly y: number;
	readonly rows: readonly string[];
	/** Mirroring moves this layer without flipping its pixels, so lettering like the hat's "C" never reads backwards. */
	readonly keepOrientation?: true;
};

/** Layers drawn bottom to top. */
export type Pose = readonly Layer[];

/** A composed canvas: CANVAS_HEIGHT strings of CANVAS_WIDTH layer characters. */
export type Grid = readonly string[];
