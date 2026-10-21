// Pure geometry for the growth chart, so the scaling is testable without rendering.
import { daysBetween } from "../lib/dates";
import type { HeightPoint } from "./copy";

export interface ChartBox {
  width: number;
  height: number;
  /** Room kept around the plot so markers aren't clipped. */
  pad: number;
}

export interface ChartGeometry {
  /** In the box's units; x by date, y up for taller. */
  points: { x: number; y: number }[];
  /** The y of the lowest and highest heights, where the two gridlines sit. */
  yMin: number;
  yMax: number;
  min: number;
  max: number;
}

/**
 * Places points by time (not by index, so a long gap reads as one) and height. The y range is the
 * data's own min to max: two labelled gridlines then mark exactly the lowest and highest heights.
 * A flat series sits in the middle.
 */
export function chartGeometry(series: HeightPoint[], { width, height, pad }: ChartBox): ChartGeometry {
  const heights = series.map((p) => p.height);
  const min = Math.min(...heights);
  const max = Math.max(...heights);
  const first = series[0]?.on ?? "";
  const span = series.length > 1 ? daysBetween(first, series.at(-1)?.on ?? first) : 0;
  const x = (on: string) => (span === 0 ? width / 2 : pad + (daysBetween(first, on) / span) * (width - 2 * pad));
  const y = (h: number) => (max === min ? height / 2 : height - pad - ((h - min) / (max - min)) * (height - 2 * pad));
  return {
    points: series.map((p) => ({ x: x(p.on), y: y(p.height) })),
    yMin: y(min),
    yMax: y(max),
    min,
    max,
  };
}
