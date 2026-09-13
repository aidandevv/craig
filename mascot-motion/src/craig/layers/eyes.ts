import type { Layer } from "../layer.ts";

export const eyes: Layer = {
	name: "eyes",
	x: 20,
	y: 21,
	rows: [
		"##            ##",
		"##            ##",
		"##            ##",
	],
};

/** Both eyes shifted one pixel toward the horizon. */
export const eyesLookRight: Layer = {
	name: "eyesLookRight",
	x: 21,
	y: 21,
	rows: [
		"##            ##",
		"##            ##",
		"##            ##",
	],
};

/** His far eye squeezed shut; the near eye hides behind the eyepiece. */
export const eyesSquintLeft: Layer = {
	name: "eyesSquintLeft",
	x: 19,
	y: 21,
	rows: [
		"               ##",
		"####           ##",
		"               ##",
	],
};

/** Both eyes closed: the middle drawing of a blink. */
export const eyesClosed: Layer = {
	name: "eyesClosed",
	x: 19,
	y: 22,
	rows: [
		"####          ####",
	],
};

/** Eyes half shut: the drawing on either side of a blink. */
export const eyesHalf: Layer = {
	name: "eyesHalf",
	x: 20,
	y: 22,
	rows: [
		"##            ##",
		"##            ##",
	],
};
