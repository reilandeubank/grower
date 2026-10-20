import type { QueryClient } from "@tanstack/react-query";

export const plantsKey = ["plants"] as const;
export const locationsKey = ["locations"] as const;
export const todayKey = ["today"] as const;
export const speciesKey = ["species"] as const;

/** A plant's tasks and history sit under its key, so refreshing the plant refreshes them too. */
export const plantTasksKey = (plantId: string) => [...plantsKey, plantId, "tasks"] as const;
export const plantHistoryKey = (plantId: string) => [...plantsKey, plantId, "history"] as const;
export const plantPhotosKey = (plantId: string) => [...plantsKey, plantId, "photos"] as const;

/**
 * Plant writes change location plant counts, and location deletes change plants, so refresh both.
 * Today shows plant names and leaves out archived plants, so it refreshes as well.
 * `refetchType: "none"` only marks them stale, for a delete that is about to leave the page whose
 * query would otherwise refetch into a 404.
 */
export function invalidatePlantsAndLocations(queryClient: QueryClient, refetchType: "active" | "none" = "active") {
  return Promise.all(
    [plantsKey, locationsKey, todayKey].map((queryKey) => queryClient.invalidateQueries({ queryKey, refetchType })),
  );
}
