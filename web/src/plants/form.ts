import type { LightLevel, Plant, PlantInput, PotType, WindowDistance } from "../api/client";

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
  light: LightLevel | "";
  windowDistance: WindowDistance | "";
  growLight: boolean;
  /** Kept while the switch is off, so turning it back on restores it. */
  growLightHours: string;
}

export type PlantErrors = Partial<Record<"name" | "potSize" | "growLightHours", string>>;

export const POT_SIZE_MIN = 0.5;
export const POT_SIZE_MAX = 48;
export const GROW_LIGHT_DEFAULT_HOURS = 12;

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
    light: plant?.light ?? "",
    windowDistance: plant?.window_distance ?? "",
    growLight: plant?.grow_light_hours != null,
    growLightHours: String(plant?.grow_light_hours ?? GROW_LIGHT_DEFAULT_HOURS),
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
  if (values.growLight) {
    const hours = Number(values.growLightHours);
    if (!Number.isInteger(hours) || hours < 1 || hours > 24) errors.growLightHours = "Use whole hours from 1 to 24.";
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
    light: values.light || null,
    window_distance: values.windowDistance || null,
    grow_light_hours: values.growLight ? Number(values.growLightHours) : null,
  };
}
