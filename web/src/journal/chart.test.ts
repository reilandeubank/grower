import { describe, expect, it } from "vitest";
import { chartGeometry } from "./chart";

const box = { width: 100, height: 50, pad: 5 };

describe("chartGeometry", () => {
  it("places points by date and height inside the padding", () => {
    const g = chartGeometry(
      [
        { on: "2026-01-01", height: 10 },
        { on: "2026-01-04", height: 20 },
        { on: "2026-01-11", height: 15 },
      ],
      box,
    );
    // x: 0, 3 and 10 days of a 10-day span over 90 units; y: tallest at the top.
    expect(g.points).toEqual([
      { x: 5, y: 45 },
      { x: 32, y: 5 },
      { x: 95, y: 25 },
    ]);
    expect(g).toMatchObject({ min: 10, max: 20, yMin: 45, yMax: 5 });
  });

  it("centres a flat series and a single day", () => {
    const g = chartGeometry(
      [
        { on: "2026-01-01", height: 12 },
        { on: "2026-01-01", height: 12 },
      ],
      box,
    );
    expect(g.points).toEqual([
      { x: 50, y: 25 },
      { x: 50, y: 25 },
    ]);
    expect(g.yMin).toBe(g.yMax);
  });
});
