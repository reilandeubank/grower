import type { Plant, PlantInput, PotType } from "../api/client";

/** The plant form's fields as the inputs hold them: strings, "" for not set. */
export interface PlantValues {
  name: string;
  species: string;
  /** The linked saved species, or "" for free text only. */
  speciesId: string;
  locationId: string;
  acquiredOn: string;
  notes: string;
  potType: PotType | "";
  potSize: string;
  drainage: "" | "yes" | "no";
  soil: string;
}

export type PlantErrors = Partial<Record<"name" | "potSize", string>>;

export const POT_SIZE_MIN = 0.5;
export const POT_SIZE_MAX = 48;

export function plantValues(plant?: Plant): PlantValues {
  return {
    name: plant?.name ?? "",
    species: plant?.species ?? "",
    speciesId: plant?.species_id ?? "",
    locationId: plant?.location_id ?? "",
    acquiredOn: plant?.acquired_on ?? "",
    notes: plant?.notes ?? "",
    potType: plant?.pot_type ?? "",
    potSize: plant?.pot_size_in == null ? "" : String(plant.pot_size_in),
    drainage: plant?.drainage == null ? "" : plant.drainage ? "yes" : "no",
    soil: plant?.soil ?? "",
  };
}

export function validatePlant(values: PlantValues): PlantErrors {
  const errors: PlantErrors = {};
  if (!values.name.trim()) errors.name = "Give your plant a name.";
  if (values.potSize.trim()) {
    const size = Number(values.potSize);
    if (!Number.isFinite(size) || size < POT_SIZE_MIN || size > POT_SIZE_MAX) {
      errors.potSize = `Use a size from ${POT_SIZE_MIN} to ${POT_SIZE_MAX} inches.`;
    }
  }
  return errors;
}

/** The request body for valid values. */
export function plantInput(values: PlantValues): PlantInput {
  return {
    name: values.name.trim(),
    species: values.species.trim(),
    species_id: values.speciesId || null,
    location_id: values.locationId || null,
    acquired_on: values.acquiredOn || null,
    notes: values.notes,
    pot_type: values.potType || null,
    pot_size_in: values.potSize.trim() ? Number(values.potSize) : null,
    drainage: values.drainage === "" ? null : values.drainage === "yes",
    soil: values.soil.trim(),
  };
}
