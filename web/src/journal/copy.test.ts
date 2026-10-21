import { describe, expect, it } from "vitest";
import type { Measurement } from "../api/client";
import { chartSummary, growthLine, heightPoints, inches, sizeLine } from "./copy";

const m = (on: string, height_in: number | null): Measurement => ({
  id: on,
  plant_id: "p1",
  measured_on: on,
  height_in,
  width_in: null,
  stage: null,
  note: "",
  photo: null,
});

describe("growth copy", () => {
  it("shows inches with one decimal at most", () => {
    expect(inches(12)).toBe("12 in");
    expect(inches(12.5)).toBe("12.5 in");
    expect(inches(12.46)).toBe("12.5 in");
    expect(inches(12.04)).toBe("12 in");
  });

  it("names size and stage, leaving out what wasn't measured", () => {
    expect(sizeLine({ height_in: 18, width_in: 14, stage: "mature" })).toBe("18 in tall · 14 in wide · Mature");
    expect(sizeLine({ height_in: 12.5, width_in: null, stage: null })).toBe("12.5 in tall");
    expect(sizeLine({ height_in: null, width_in: null, stage: "seedling" })).toBe("Seedling");
    expect(sizeLine({ height_in: null, width_in: null, stage: null })).toBe("");
  });

  it("keeps the plant's meta line short", () => {
    expect(growthLine(null)).toBeNull();
    expect(growthLine({ height_in: 18, width_in: 14, stage: "mature", measured_on: "2026-10-01" })).toBe("18 in tall · Mature");
    expect(growthLine({ height_in: null, width_in: 14, stage: null, measured_on: "2026-10-01" })).toBe("14 in wide");
    expect(growthLine({ height_in: null, width_in: null, stage: "young", measured_on: "2026-10-01" })).toBe("Young");
    expect(growthLine({ height_in: null, width_in: null, stage: null, measured_on: "2026-10-01" })).toBeNull();
  });

  it("orders heights oldest first and skips measurements without one", () => {
    expect(heightPoints([m("2026-10-01", 18), m("2026-06-01", null), m("2026-03-02", 12)])).toEqual([
      { on: "2026-03-02", height: 12 },
      { on: "2026-10-01", height: 18 },
    ]);
  });

  it("keeps same-day heights in the order they were added", () => {
    // The API lists newest first, ties included.
    const later = { ...m("2026-10-21", 15), id: "b" };
    expect(heightPoints([later, m("2026-10-21", 12)]).map((p) => p.height)).toEqual([12, 15]);
  });

  it("summarises the chart in words", () => {
    const grew = [
      { on: "2026-03-02", height: 12 },
      { on: "2026-10-01", height: 18 },
    ];
    expect(chartSummary(grew, "2026-10-21")).toBe("Height grew from 12 in to 18 in since March");
    expect(chartSummary(grew, "2027-01-05")).toBe("Height grew from 12 in to 18 in since March 2026");
    const shrank = [
      { on: "2026-03-02", height: 18 },
      { on: "2026-10-01", height: 12 },
    ];
    expect(chartSummary(shrank, "2026-10-21")).toBe(
      "Height went from 18 in to 12 in since March",
    );
    expect(chartSummary([{ on: "2026-03-02", height: 12 }, { on: "2026-04-02", height: 12 }], "2026-10-21")).toBe(
      "Height held at 12 in since March",
    );
  });
});
