// Species copy: the care guide's lines, where a species came from, and what applying it did.
import type { CareTask, LightLevel, Species, SpeciesLookup, SpeciesSource } from "../api/client";
import { readingLine } from "../care/copy";

export const SOURCE_LABELS: Record<SpeciesSource, string> = {
  openplantbook: "Open Plantbook",
  perenual: "Perenual",
  manual: "Added by you",
};

/** Why the online search came back short, as notes under its results. */
export function lookupNotes(result: SpeciesLookup): string[] {
  if (result.providers.length === 0) return ["Online lookup isn't set up. Ask whoever runs grower to add API keys."];
  const notes = result.providers
    .filter((p) => !p.ok)
    .map((p) => `${SOURCE_LABELS[p.source]} didn't answer.${p.detail ? ` ${p.detail}` : ""}`);
  if (result.items.length === 0 && result.providers.some((p) => p.ok)) notes.unshift("No matches online.");
  return notes;
}

/** Light as the care guide says it; the location form's shorter labels read oddly on their own. */
export const LIGHT_LINES: Record<LightLevel, string> = {
  low: "Low light",
  medium: "Medium light",
  bright_indirect: "Bright, indirect light",
  direct: "Direct sun",
};

export type GuideLine = { kind: "water" | "light" | "waterAt"; text: string };

/** "Water about every 7 days", "Bright, indirect light", "Water at 3 · Dry": only what's known. */
export function careGuideLines(s: Pick<Species, "water_interval_days" | "light" | "water_at">): GuideLine[] {
  const lines: GuideLine[] = [];
  const days = s.water_interval_days;
  if (days !== null) {
    lines.push({ kind: "water", text: days === 1 ? "Water about every day" : `Water about every ${days} days` });
  }
  if (s.light !== null) lines.push({ kind: "light", text: LIGHT_LINES[s.light] });
  if (s.water_at !== null) lines.push({ kind: "waterAt", text: `Water at ${readingLine(s.water_at)}` });
  return lines;
}

/** "Added by you", "Edited by you" or "From Perenual". */
export function sourceLine(s: Pick<Species, "source" | "user_edited">): string {
  if (s.source === "manual") return "Added by you";
  if (s.user_edited) return "Edited by you";
  return `From ${SOURCE_LABELS[s.source]}`;
}

/** Whether applying has anything to set: the server only touches the water task. */
export function hasCareDefaults(s: Pick<Species, "water_interval_days" | "water_at">): boolean {
  return s.water_interval_days !== null || s.water_at !== null;
}

/** The toast after applying: a new task, or what changed on the existing one. */
export function applyToast(before: CareTask[] | undefined, after: CareTask[]): string {
  const water = after.find((t) => t.kind === "water");
  if (!water) return "Care schedule updated.";
  if (before && !before.some((t) => t.kind === "water")) return "Water task added.";
  if (water.moisture_check) return `Soil check set to water at ${readingLine(water.water_at)}.`;
  const days = water.interval_days;
  return days === 1 ? "Watering set to every day." : `Watering set to every ${days} days.`;
}
