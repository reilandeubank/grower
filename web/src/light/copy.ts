// Light copy: windows, window distance and the plant page's light line.
import type { Direction, Plant, Window, WindowDistance } from "../api/client";
import { LIGHT_LINES } from "../species/copy";

/** Compass order, clockwise from north. */
export const DIRECTIONS: Direction[] = ["n", "ne", "e", "se", "s", "sw", "w", "nw"];

export const DIRECTION_NAMES: Record<Direction, string> = {
  n: "North",
  ne: "Northeast",
  e: "East",
  se: "Southeast",
  s: "South",
  sw: "Southwest",
  w: "West",
  nw: "Northwest",
};

export const DISTANCE_LABELS: Record<WindowDistance, string> = {
  at_window: "At the window",
  near: "Near it (about 3 ft)",
  across_room: "Across the room",
};

const DISTANCE_PHRASES: Record<WindowDistance, string> = {
  at_window: "at the window",
  near: "near the window",
  across_room: "across the room",
};

/** "1 N · 2 E", in compass order; "" with no windows. */
export function windowsLine(windows: Window[]): string {
  return DIRECTIONS.flatMap((d) => {
    const count = windows.find((w) => w.direction === d)?.count ?? 0;
    return count > 0 ? [`${count} ${d.toUpperCase()}`] : [];
  }).join(" · ");
}

/**
 * What light the plant gets and why: "Medium light · near the window", "Bright, indirect light ·
 * grow light 12 h", "Direct sun · set by you". The room is named just above it on the plant page.
 * Null when nothing is known.
 */
export function lightLine(plant: Pick<Plant, "effective_light" | "window_distance" | "grow_light_hours">): string | null {
  const light = plant.effective_light;
  if (!light) return null;
  let why: string | null = null;
  if (light.source === "plant") why = "set by you";
  else if (light.source === "grow_light") why = `grow light ${plant.grow_light_hours} h`;
  else if (plant.window_distance) why = DISTANCE_PHRASES[plant.window_distance];
  return why ? `${LIGHT_LINES[light.level]} · ${why}` : LIGHT_LINES[light.level];
}
