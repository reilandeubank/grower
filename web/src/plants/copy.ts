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
