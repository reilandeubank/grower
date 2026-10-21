// Shared test data for the species tests.
import type { CareTask, Plant, Species } from "../api/client";

export const monstera: Species = {
  id: "s1",
  scientific_name: "Monstera deliciosa",
  common_name: "Swiss cheese plant",
  source: "perenual",
  source_ref: "1",
  water_interval_days: 7,
  light: "bright_indirect",
  water_at: 3,
  notes: "",
  fetched_at: "2026-10-14T12:00:00Z",
  user_edited: false,
  plant_count: 1,
};

export const plant: Plant = {
  id: "p1",
  name: "Monty",
  species: "Monstera deliciosa",
  species_id: "s1",
  location_id: null,
  acquired_on: null,
  notes: "",
  archived: false,
  cover: null,
  created_at: "2026-10-14T12:00:00Z",
  pot_type: null,
  pot_size_in: null,
  drainage: null,
  soil: "",
  light: null,
  window_distance: null,
  grow_light_hours: null,
  effective_light: null,
  growth: null,
};

export const waterTask: CareTask = {
  id: "t1",
  plant_id: "p1",
  kind: "water",
  label: "",
  interval_days: 7,
  anchor: "from_completion",
  start_on: "2026-10-14",
  seasonal: false,
  moisture_check: false,
  water_at: 3,
  last_reading: null,
  drying_days: null,
  current_interval_days: 7,
  next_due: "2026-10-14",
  last_done: null,
};
