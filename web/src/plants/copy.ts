import type { Plant, PotType } from "../api/client";

export const POT_LABELS: Record<PotType, string> = {
  plastic: "Plastic",
  terracotta: "Terracotta",
  ceramic: "Ceramic",
  fabric: "Fabric",
  self_watering: "Self-watering",
  other: "Other",
};

/** "6 in terracotta pot · drains · Aroid mix", or null when nothing about the pot is recorded. */
export function potLine(plant: Pick<Plant, "pot_type" | "pot_size_in" | "drainage" | "soil">): string | null {
  const size = plant.pot_size_in === null ? null : `${plant.pot_size_in} in`;
  // "Other" says nothing a reader can use, so it reads as just "pot".
  const type = plant.pot_type && plant.pot_type !== "other" ? POT_LABELS[plant.pot_type].toLowerCase() : null;
  const pot = size || plant.pot_type ? [size, type, "pot"].filter(Boolean).join(" ") : null;
  const drainage = plant.drainage === null ? null : plant.drainage ? "drains" : "no drainage";
  const parts = [pot, drainage, plant.soil.trim() || null].filter(Boolean);
  if (parts.length === 0) return null;
  const line = parts.join(" · ");
  return line.charAt(0).toUpperCase() + line.slice(1);
}

/** "When was it last repotted?": each answer's first repot reminder, in days from today. Repots are yearly. */
export const REPOT_OPTIONS = [
  { label: "Never, it's in its nursery pot", dueInDays: 0 },
  { label: "In the last month", dueInDays: 350 },
  { label: "A few months ago", dueInDays: 245 },
  { label: "A year ago or more", dueInDays: 0 },
] as const;

/** The toast for a plant that saved without its photo or repot reminder; null when nothing failed. */
export function addedButMessage(name: string, photoFailed: boolean, repotFailed: boolean): string | null {
  if (photoFailed && repotFailed) {
    return `Added ${name}, but the photo and repot reminder didn't save. Add them from Photos and Care.`;
  }
  if (photoFailed) return `Added ${name}, but the photo didn't upload. Add it from Photos.`;
  if (repotFailed) return `Added ${name}, but the repot reminder didn't save. Add it from Care.`;
  return null;
}
