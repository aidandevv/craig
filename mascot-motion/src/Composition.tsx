import { Composition, Still } from "remotion";
import { CraigSprite } from "./craig/CraigSprite";
import { sprites } from "./craig/poses";
import { idleLoopComposition } from "./IdleLoop";
import { lookoutSearchComposition } from "./LookoutSearch";
import { peekSearchComposition } from "./PeekSearch";
import { telescopeSearchComposition } from "./TelescopeSearch";

/** "telescope-side-right" becomes "Craig-Telescope-Side-Right". */
const compositionId = (name: string): string =>
	`Craig-${name
		.split("-")
		.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
		.join("-")}`;

export const CraigCompositions = () => (
	<>
		{sprites.map(([name, pose]) => (
			<Still
				component={CraigSprite}
				defaultProps={{ pose, scale: 8 }}
				height={640}
				id={compositionId(name)}
				key={name}
				width={448}
			/>
		))}
		<Composition {...telescopeSearchComposition} />
		<Composition {...lookoutSearchComposition} />
		<Composition {...peekSearchComposition} />
		<Composition {...idleLoopComposition} />
	</>
);
