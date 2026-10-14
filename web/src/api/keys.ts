import type { QueryClient } from "@tanstack/react-query";

export const plantsKey = ["plants"] as const;
export const locationsKey = ["locations"] as const;

/**
 * Plant writes change location plant counts, and location deletes change plants, so refresh both.
 * `refetchType: "none"` only marks them stale, for a delete that is about to leave the page whose
 * query would otherwise refetch into a 404.
 */
export function invalidatePlantsAndLocations(queryClient: QueryClient, refetchType: "active" | "none" = "active") {
  return Promise.all(
    [plantsKey, locationsKey].map((queryKey) => queryClient.invalidateQueries({ queryKey, refetchType })),
  );
}
