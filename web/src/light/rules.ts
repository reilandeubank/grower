// The light rules the screens need ahead of the server: a mirror of internal/light (PLAN §5.6).
import type { Direction, LightLevel, Settings, Window } from "../api/client";

export type Hemisphere = Settings["hemisphere"];

/** Darkest first, so an index is a step count. */
export const LIGHT_ORDER: LightLevel[] = ["low", "medium", "bright_indirect", "direct"];

const step = (level: LightLevel) => LIGHT_ORDER.indexOf(level);

// What one window facing each way lets in. The south mirrors the north.
const NORTH: Record<Direction, LightLevel> = {
  s: "direct",
  se: "bright_indirect",
  sw: "bright_indirect",
  e: "bright_indirect",
  w: "bright_indirect",
  n: "medium",
  ne: "medium",
  nw: "medium",
};
const MIRROR: Record<Direction, Direction> = { n: "s", ne: "se", e: "e", se: "ne", s: "n", sw: "nw", w: "w", nw: "sw" };

/** The room's light estimated from its windows: the best window decides; two or more lift medium. */
export function suggestLight(windows: Window[], hemisphere: Hemisphere): LightLevel | null {
  const present = windows.filter((w) => w.count > 0);
  if (present.length === 0) return null;
  const levels = present.map((w) => NORTH[hemisphere === "south" ? MIRROR[w.direction] : w.direction]);
  const best = levels.reduce((a, b) => (step(b) > step(a) ? b : a));
  const total = present.reduce((n, w) => n + w.count, 0);
  return best === "medium" && total >= 2 ? "bright_indirect" : best;
}

/** True when the plant gets two or more steps less light than its species prefers. */
export function needsMoreLight(level: LightLevel | undefined, preferred: LightLevel | null): boolean {
  if (!level || !preferred) return false;
  return step(preferred) - step(level) >= 2;
}
