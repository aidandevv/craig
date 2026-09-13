import type { Pose } from "./layer.ts";
import { arms } from "./layers/arms.ts";
import { brows } from "./layers/brows.ts";
import { coat } from "./layers/coat.ts";
import { epaulettes } from "./layers/epaulettes.ts";
import { eyes } from "./layers/eyes.ts";
import { badge, hat } from "./layers/hat.ts";
import { head } from "./layers/head.ts";
import { legs } from "./layers/legs.ts";
import { mouth } from "./layers/mouth.ts";
import { mustache } from "./layers/mustache.ts";
import { nose } from "./layers/nose.ts";

/** Craig's neutral, prop-free pose, bottom to top. Every state starts from this. */
export const basePose: Pose = [legs, arms, coat, epaulettes, head, mouth, mustache, nose, eyes, brows, hat, badge];
