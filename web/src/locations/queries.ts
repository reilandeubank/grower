import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, unwrap, type LightLevel, type Location, type LocationInput } from "../api/client";
import { invalidatePlantsAndLocations, locationsKey } from "../api/keys";

export const LIGHT_LABELS: Record<LightLevel, string> = {
  low: "Low light",
  medium: "Medium light",
  bright_indirect: "Bright, indirect",
  direct: "Direct sun",
};

export function useLocations() {
  return useQuery({
    queryKey: locationsKey,
    queryFn: async () => unwrap(await api.GET("/locations")).items,
  });
}

/** Creates when `id` is absent, otherwise replaces. */
export function useSaveLocation() {
  const queryClient = useQueryClient();
  return useMutation<Location, Error, { id?: string; body: LocationInput }>({
    mutationFn: async ({ id, body }) =>
      unwrap(
        id
          ? await api.PUT("/locations/{id}", { params: { path: { id } }, body })
          : await api.POST("/locations", { body }),
      ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: locationsKey }),
  });
}

export function useDeleteLocation() {
  const queryClient = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: async (id) => {
      const { response, error } = await api.DELETE("/locations/{id}", { params: { path: { id } } });
      if (!response.ok) throw new ApiError(response.status, error);
    },
    onSuccess: () => invalidatePlantsAndLocations(queryClient),
  });
}
