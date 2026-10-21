import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { api, ApiError, unwrap, type Measurement, type MeasurementInput } from "../api/client";
import { plantMeasurementsKey, plantsKey } from "../api/keys";

export function usePlantMeasurements(plantId: string) {
  return useQuery({
    queryKey: plantMeasurementsKey(plantId),
    queryFn: async () =>
      unwrap(await api.GET("/plants/{id}/measurements", { params: { path: { id: plantId } } })).items,
  });
}

/** Measurements sit under the plant's key, which also refreshes the plant's growth line. */
function refreshPlant(queryClient: QueryClient, plantId: string) {
  return queryClient.invalidateQueries({ queryKey: [...plantsKey, plantId] });
}

/** Creates when `id` is absent, otherwise replaces the measurement. */
export function useSaveMeasurement(plantId: string) {
  const queryClient = useQueryClient();
  return useMutation<Measurement, Error, { id?: string | undefined; body: MeasurementInput }>({
    mutationFn: async ({ id, body }) =>
      unwrap(
        id
          ? await api.PUT("/measurements/{id}", { params: { path: { id } }, body })
          : await api.POST("/plants/{id}/measurements", { params: { path: { id: plantId } }, body }),
      ),
    onSuccess: () => refreshPlant(queryClient, plantId),
  });
}

export function useDeleteMeasurement(plantId: string) {
  const queryClient = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: async (id) => {
      const { response, error } = await api.DELETE("/measurements/{id}", { params: { path: { id } } });
      if (!response.ok) throw new ApiError(response.status, error);
    },
    onSuccess: () => refreshPlant(queryClient, plantId),
  });
}
