import { describe, expect, it } from "vitest";
import type { EffectiveLight, Plant } from "../api/client";
import { lightLine, windowsLine } from "./copy";

type LightFields = Pick<Plant, "effective_light" | "window_distance" | "grow_light_hours">;
const plant = (effective_light: EffectiveLight | null, more: Partial<LightFields> = {}): LightFields => ({
  effective_light,
  window_distance: null,
  grow_light_hours: null,
  ...more,
});

describe("lightLine", () => {
  it("is null when nothing is known", () => {
    expect(lightLine(plant(null))).toBeNull();
  });

  it("gives the distance from the window for room light, set or estimated", () => {
    expect(lightLine(plant({ level: "bright_indirect", source: "location" }, { window_distance: "near" }))).toBe(
      "Bright, indirect light · near the window",
    );
    expect(lightLine(plant({ level: "low", source: "windows" }, { window_distance: "across_room" }))).toBe(
      "Low light · across the room",
    );
    expect(lightLine(plant({ level: "direct", source: "windows" }, { window_distance: "at_window" }))).toBe(
      "Direct sun · at the window",
    );
    expect(lightLine(plant({ level: "medium", source: "location" }))).toBe("Medium light");
  });

  it("credits a grow light with its hours", () => {
    expect(lightLine(plant({ level: "bright_indirect", source: "grow_light" }, { grow_light_hours: 12 }))).toBe(
      "Bright, indirect light · grow light 12 h",
    );
  });

  it("says when the light was set by hand", () => {
    expect(lightLine(plant({ level: "direct", source: "plant" }))).toBe("Direct sun · set by you");
  });
});

describe("windowsLine", () => {
  it("lists windows in compass order", () => {
    expect(
      windowsLine([
        { direction: "e", count: 1 },
        { direction: "n", count: 2 },
      ]),
    ).toBe("2 N · 1 E");
    expect(windowsLine([])).toBe("");
  });
});
