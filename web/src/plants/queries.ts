import { useRef } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, unwrap, type Plant, type PlantInput, type PlantUpdate, type SpeciesInput } from "../api/client";
import { invalidatePlantsAndLocations, plantsKey, speciesKey } from "../api/keys";
import type { PreparedPhoto } from "../photos/prepare";
import { uploadPhoto } from "../photos/queries";

/** Active plants by default; `archived` lists only archived ones. */
export function usePlants(archived = false) {
  return useQuery({
    queryKey: [...plantsKey, { archived }],
    queryFn: async () => unwrap(await api.GET("/plants", { params: { query: { archived } } })).items,
  });
}

export function usePlant(id: string) {
  return useQuery({
    queryKey: [...plantsKey, id],
    queryFn: async () => unwrap(await api.GET("/plants/{id}", { params: { path: { id } } })),
    enabled: !wasDeleted(id),
    // A missing plant is an answer; don't retry it.
    retry: (count, error) => !(error instanceof ApiError && error.status === 404) && count < 1,
  });
}

/** A plant's writable fields with some replaced, for a PUT (which replaces them all). */
export function plantUpdate(plant: Plant, changes: Partial<PlantUpdate> = {}): PlantUpdate {
  const {
    name,
    species,
    species_id,
    location_id,
    acquired_on,
    notes,
    archived,
    pot_type,
    pot_size_in,
    drainage,
    soil,
    light,
    window_distance,
    grow_light_hours,
  } = plant;
  return {
    name,
    species,
    species_id,
    location_id,
    acquired_on,
    notes,
    archived,
    pot_type,
    pot_size_in,
    drainage,
    soil,
    light,
    window_distance,
    grow_light_hours,
    ...changes,
  };
}

/** Replaces a plant's fields (archiving included). */
export function useSavePlant() {
  const queryClient = useQueryClient();
  return useMutation<Plant, Error, { id: string; body: PlantUpdate }>({
    mutationFn: async ({ id, body }) => unwrap(await api.PUT("/plants/{id}", { params: { path: { id } }, body })),
    onSuccess: (plant) => {
      queryClient.setQueryData([...plantsKey, plant.id], plant);
      return invalidatePlantsAndLocations(queryClient);
    },
  });
}

export interface CreatePlant {
  body: PlantInput;
  /** A species to create and link first (an identified one that isn't saved yet). */
  newSpecies?: SpeciesInput | null;
  /** A prepared photo to upload once the plant exists; the first photo becomes the cover. */
  photo?: PreparedPhoto | null;
}

export interface CreatedPlant {
  plant: Plant;
  /** The plant was saved but its photo wasn't; the photo can be added again from its page. */
  photoError: Error | null;
}

/** Creates a plant: its new species first, then the plant, then its photo. */
export function useCreatePlant() {
  const queryClient = useQueryClient();
  // Species made by an attempt whose plant then failed, by scientific name: a retry links them
  // rather than creating a duplicate.
  const made = useRef(new Map<string, string>());
  return useMutation<CreatedPlant, Error, CreatePlant>({
    mutationFn: async ({ body, newSpecies, photo }) => {
      let speciesId = body.species_id ?? null;
      if (newSpecies) {
        const name = newSpecies.scientific_name;
        speciesId = made.current.get(name) ?? unwrap(await api.POST("/species", { body: newSpecies })).id;
        made.current.set(name, speciesId);
      }
      const plant = unwrap(await api.POST("/plants", { body: { ...body, species_id: speciesId } }));
      let photoError: Error | null = null;
      if (photo) {
        try {
          await uploadPhoto(plant.id, photo);
        } catch (error) {
          photoError = error instanceof Error ? error : new Error(String(error));
        }
      }
      return { plant, photoError };
    },
    onSettled: (_data, _error, { newSpecies }) =>
      Promise.all([
        invalidatePlantsAndLocations(queryClient),
        newSpecies && queryClient.invalidateQueries({ queryKey: speciesKey }),
      ]),
  });
}

// Plants deleted during this visit. History still holds their pages, and Back to one should
// skip it rather than show "Plant not found".
const deleted = new Set<string>();

export function wasDeleted(id: string): boolean {
  return deleted.has(id);
}

export function useDeletePlant() {
  const queryClient = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: async (id) => {
      const { response, error } = await api.DELETE("/plants/{id}", { params: { path: { id } } });
      if (!response.ok) throw new ApiError(response.status, error);
    },
    onSuccess: (_data, id) => {
      deleted.add(id);
      return invalidatePlantsAndLocations(queryClient, "none");
    },
  });
}
