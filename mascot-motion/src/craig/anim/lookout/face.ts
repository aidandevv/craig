import type { Layer } from "../../layer.ts";
import { brows } from "../../layers/brows.ts";
import { eyes } from "../../layers/eyes.ts";

/** Both pupils slid along the eye line. 0 is level; larger looks further out toward the horizon. */
export const gaze = (offset: number): Layer => ({ ...eyes, name: `lookoutGaze${offset}`, x: eyes.x + offset });

/** His brow lifted a pixel, the way it goes when something turns up out there. */
export const browsLifted: Layer = { ...brows, name: "lookoutBrowsLifted", y: brows.y - 1 };
