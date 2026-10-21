// Growth copy: sizes in inches with at most one decimal, stages in plain words.
import type { GrowthStage, Measurement, Plant } from "../api/client";
import { formatCivil } from "../lib/dates";

export const STAGE_LABELS: Record<GrowthStage, string> = {
  seedling: "Seedling",
  young: "Young",
  mature: "Mature",
};

export const STAGE_OPTIONS = (Object.keys(STAGE_LABELS) as GrowthStage[]).map((value) => ({
  value,
  label: STAGE_LABELS[value],
}));

/** "12 in", "12.5 in": one decimal at most, and none when it's whole. */
export function inches(n: number): string {
  return `${Math.round(n * 10) / 10} in`;
}

type Size = Pick<Measurement, "height_in" | "width_in" | "stage">;

/** "18 in tall · 14 in wide · Mature", leaving out what wasn't measured; "" when nothing was. */
export function sizeLine({ height_in, width_in, stage }: Size): string {
  return [
    height_in === null ? null : `${inches(height_in)} tall`,
    width_in === null ? null : `${inches(width_in)} wide`,
    stage === null ? null : STAGE_LABELS[stage],
  ]
    .filter(Boolean)
    .join(" · ");
}

/** The plant's meta line: "18 in tall · Mature". Height stands for size, width only without it. */
export function growthLine(growth: Plant["growth"]): string | null {
  if (!growth) return null;
  const size =
    growth.height_in !== null
      ? `${inches(growth.height_in)} tall`
      : growth.width_in !== null
        ? `${inches(growth.width_in)} wide`
        : null;
  const line = [size, growth.stage && STAGE_LABELS[growth.stage]].filter(Boolean).join(" · ");
  return line || null;
}

export interface HeightPoint {
  /** YYYY-MM-DD */
  on: string;
  height: number;
}

/** Measurements with a height, oldest first, for the chart. Same-day ones keep the order they were added in. */
export function heightPoints(measurements: Measurement[]): HeightPoint[] {
  // The list is newest first (ties too), so reversing it before the stable sort keeps ties in order.
  return [...measurements]
    .reverse()
    .filter((m): m is Measurement & { height_in: number } => m.height_in !== null)
    .map((m) => ({ on: m.measured_on, height: m.height_in }))
    .sort((a, b) => (a.on < b.on ? -1 : a.on > b.on ? 1 : 0));
}

/** The chart's words for screen readers: "Height grew from 12 in to 18 in since March". */
export function chartSummary(points: HeightPoint[], today: string): string {
  const first = points[0];
  const last = points.at(-1);
  if (!first || !last) return "No heights yet";
  const sameYear = first.on.slice(0, 4) === today.slice(0, 4);
  const since = formatCivil(first.on, { month: "long", ...(sameYear ? {} : { year: "numeric" }) });
  if (last.height === first.height) return `Height held at ${inches(last.height)} since ${since}`;
  const verb = last.height > first.height ? "grew" : "went";
  return `Height ${verb} from ${inches(first.height)} to ${inches(last.height)} since ${since}`;
}
