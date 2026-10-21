import { describe, expect, it } from "vitest";
import type { Direction, LightLevel, Window } from "../api/client";
import { needsMoreLight, suggestLight, type Hemisphere } from "./rules";

const one = (direction: Direction, count = 1): Window[] => [{ direction, count }];

describe("suggestLight (PLAN §5.6)", () => {
  const cases: [Hemisphere, Direction, LightLevel][] = [
    ["north", "s", "direct"],
    ["north", "e", "bright_indirect"],
    ["north", "w", "bright_indirect"],
    ["north", "se", "bright_indirect"],
    ["north", "sw", "bright_indirect"],
    ["north", "n", "medium"],
    ["north", "ne", "medium"],
    ["north", "nw", "medium"],
    ["south", "n", "direct"],
    ["south", "e", "bright_indirect"],
    ["south", "w", "bright_indirect"],
    ["south", "ne", "bright_indirect"],
    ["south", "nw", "bright_indirect"],
    ["south", "s", "medium"],
    ["south", "se", "medium"],
    ["south", "sw", "medium"],
  ];
  it.each(cases)("%s hemisphere, one %s window: %s", (hemisphere, direction, level) => {
    expect(suggestLight(one(direction), hemisphere)).toBe(level);
  });

  it("is null with no windows", () => {
    expect(suggestLight([], "north")).toBeNull();
    expect(suggestLight(one("s", 0), "north")).toBeNull();
  });

  it("lets the best window decide", () => {
    expect(suggestLight([...one("n"), ...one("e"), ...one("s")], "north")).toBe("direct");
  });

  it("raises medium to bright, indirect with two or more windows in total", () => {
    expect(suggestLight(one("n", 2), "north")).toBe("bright_indirect");
    expect(suggestLight([...one("n"), ...one("nw")], "north")).toBe("bright_indirect");
    expect(suggestLight([...one("s"), ...one("sw")], "south")).toBe("bright_indirect");
  });
});

describe("needsMoreLight", () => {
  it("speaks up at two or more steps below the species' light", () => {
    expect(needsMoreLight("low", "bright_indirect")).toBe(true);
    expect(needsMoreLight("medium", "direct")).toBe(true);
    expect(needsMoreLight("low", "direct")).toBe(true);
  });

  it("says nothing one step below, at or above, or when either is unknown", () => {
    expect(needsMoreLight("medium", "bright_indirect")).toBe(false);
    expect(needsMoreLight("bright_indirect", "bright_indirect")).toBe(false);
    expect(needsMoreLight("direct", "low")).toBe(false);
    expect(needsMoreLight(undefined, "direct")).toBe(false);
    expect(needsMoreLight("low", null)).toBe(false);
  });
});
