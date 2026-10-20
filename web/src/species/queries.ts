import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap, type CareTask, type Species, type SpeciesInput, type SpeciesLookup } from "../api/client";
import { plantHistoryKey, plantTasksKey, speciesKey, todayKey } from "../api/keys";

/** Saved species matching `q`; never calls a provider, so it can follow typing. */
export function useSavedSpecies(q: string) {
  return useQuery({
    queryKey: [...speciesKey, "list", q],
    queryFn: async () => unwrap(await api.GET("/species", { params: { query: { q } } })).items,
    enabled: q.length >= 2,
    // Keep the last suggestions on screen while the next ones load.
    placeholderData: keepPreviousData,
  });
}

export function useSpecies(id: string | null) {
  return useQuery({
    queryKey: [...speciesKey, id],
    queryFn: async () => unwrap(await api.GET("/species/{id}", { params: { path: { id: id ?? "" } } })),
    enabled: id !== null,
  });
}

/** Online search. A mutation because each call costs provider requests: it runs only when asked. */
export function useLookupSpecies() {
  return useMutation<SpeciesLookup, Error, string>({
    mutationFn: async (q) => unwrap(await api.GET("/species/lookup", { params: { query: { q } } })),
  });
}

export function useImportSpecies() {
  const queryClient = useQueryClient();
  return useMutation<Species, Error, { source: "openplantbook" | "perenual"; ref: string }>({
    mutationFn: async (body) => unwrap(await api.POST("/species/import", { body })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: speciesKey }),
  });
}

/** Creates when `id` is absent (a manual species), otherwise replaces and marks it edited. */
export function useSaveSpecies() {
  const queryClient = useQueryClient();
  return useMutation<Species, Error, { id?: string | undefined; body: SpeciesInput }>({
    mutationFn: async ({ id, body }) =>
      unwrap(
        id
          ? await api.PUT("/species/{id}", { params: { path: { id } }, body })
          : await api.POST("/species", { body }),
      ),
    onSuccess: (species) => {
      queryClient.setQueryData([...speciesKey, species.id], species);
      return queryClient.invalidateQueries({ queryKey: [...speciesKey, "list"] });
    },
  });
}

/** Applies the species' defaults to the plant's water task; resolves with the tasks before and after. */
export function useApplySpecies(plantId: string) {
  const queryClient = useQueryClient();
  return useMutation<{ before: CareTask[] | undefined; after: CareTask[] }, Error, void>({
    mutationFn: async () => {
      const before = queryClient.getQueryData<CareTask[]>(plantTasksKey(plantId));
      const after = unwrap(
        await api.POST("/plants/{id}/apply-species", { params: { path: { id: plantId } } }),
      ).items;
      return { before, after };
    },
    onSuccess: ({ after }) => {
      queryClient.setQueryData(plantTasksKey(plantId), after);
      return Promise.all(
        [todayKey, plantTasksKey(plantId), plantHistoryKey(plantId)].map((queryKey) =>
          queryClient.invalidateQueries({ queryKey }),
        ),
      );
    },
  });
}
