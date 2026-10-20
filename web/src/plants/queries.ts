import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, unwrap, type Plant, type PlantInput, type PlantUpdate } from "../api/client";
import { invalidatePlantsAndLocations, plantsKey } from "../api/keys";

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
  const { name, species, species_id, location_id, acquired_on, notes, archived, pot_type, pot_size_in, drainage, soil } =
    plant;
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
    ...changes,
  };
}

type SavePlant = { id: string; body: PlantUpdate } | { id?: undefined; body: PlantInput };

/** Creates when `id` is absent, otherwise replaces (archiving included). */
export function useSavePlant() {
  const queryClient = useQueryClient();
  return useMutation<Plant, Error, SavePlant>({
    mutationFn: async (vars) =>
      unwrap(
        vars.id
          ? await api.PUT("/plants/{id}", { params: { path: { id: vars.id } }, body: vars.body })
          : await api.POST("/plants", { body: vars.body }),
      ),
    onSuccess: (plant) => {
      queryClient.setQueryData([...plantsKey, plant.id], plant);
      return invalidatePlantsAndLocations(queryClient);
    },
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
