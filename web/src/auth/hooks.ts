import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, type Me } from "../api/client";
import { API_CACHE, MEDIA_CACHE } from "../lib/cacheNames";

export const meKey = ["me"] as const;

/** The signed-in user, or null when signed out (a 401 is an answer, not an error). */
export function useMe() {
  return useQuery<Me | null, Error>({
    queryKey: meKey,
    queryFn: async () => {
      const { data, error, response } = await api.GET("/me");
      if (response.status === 401) return null;
      if (error || !data) throw new ApiError(response.status, error);
      return data;
    },
    staleTime: 5 * 60 * 1000,
  });
}

export interface Credentials {
  username: string;
  password: string;
}

export function useLogin() {
  const queryClient = useQueryClient();
  return useMutation<Me, Error, Credentials>({
    mutationFn: async (body) => {
      const { data, error, response } = await api.POST("/auth/login", { body });
      if (error || !data) throw new ApiError(response.status, error);
      return data;
    },
    onSuccess: (me) => queryClient.setQueryData(meKey, me),
  });
}

export function useLogout() {
  const queryClient = useQueryClient();
  return useMutation<void, Error, void>({
    mutationFn: async () => {
      const { response } = await api.POST("/auth/logout");
      if (!response.ok) throw new ApiError(response.status);
    },
    onSuccess: () => {
      // Drop everything cached for this user, then record "signed out" so guards don't refetch.
      void clearOfflineCaches();
      queryClient.clear();
      queryClient.setQueryData(meKey, null);
    },
  });
}

/** Deletes the service worker's copies of API answers and photos (see vite.config.ts). */
export async function clearOfflineCaches() {
  if (!("caches" in window)) return;
  await Promise.all([API_CACHE, MEDIA_CACHE].map((name) => caches.delete(name)));
}
