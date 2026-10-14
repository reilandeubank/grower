import { describe, expect, it } from "vitest";
import { MAX_EDGE, targetSize } from "./prepare";

describe("targetSize", () => {
  it("fits the long side to the limit, keeping the aspect ratio", () => {
    expect(targetSize(4000, 3000)).toEqual({ width: MAX_EDGE, height: 1920 });
    expect(targetSize(3000, 4000)).toEqual({ width: 1920, height: MAX_EDGE });
    expect(targetSize(5120, 5120)).toEqual({ width: MAX_EDGE, height: MAX_EDGE });
  });

  it("never upscales", () => {
    expect(targetSize(800, 600)).toEqual({ width: 800, height: 600 });
    expect(targetSize(MAX_EDGE, 100)).toEqual({ width: MAX_EDGE, height: 100 });
  });

  it("rounds, and keeps a sliver at least a pixel", () => {
    expect(targetSize(4032, 3024)).toEqual({ width: 2560, height: 1920 });
    expect(targetSize(3333, 1111)).toEqual({ width: 2560, height: 853 });
    expect(targetSize(100_000, 10)).toEqual({ width: 2560, height: 1 });
  });
});
