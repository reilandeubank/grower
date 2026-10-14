import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, unwrap, type Me, type Settings } from "../api/client";
import { todayKey } from "../api/keys";
import { meKey } from "../auth/hooks";

export const storageKey = ["storage"] as const;

export function useStorage() {
  return useQuery({
    queryKey: storageKey,
    queryFn: async () => unwrap(await api.GET("/storage")),
  });
}

/** Saves settings; a new time zone moves "today", so Today refreshes too. */
export function useUpdateSettings() {
  const queryClient = useQueryClient();
  return useMutation<Settings, Error, Settings>({
    mutationFn: async (body) => unwrap(await api.PUT("/settings", { body })),
    onSuccess: (settings) => {
      queryClient.setQueryData<Me | null>(meKey, (me) => (me ? { ...me, settings } : me));
      return queryClient.invalidateQueries({ queryKey: todayKey });
    },
  });
}

export interface PasswordChange {
  current_password: string;
  new_password: string;
}

/** Changes the password. The server signs out every other session and keeps this one. */
export function useChangePassword() {
  return useMutation<void, Error, PasswordChange>({
    mutationFn: async (body) => {
      const { response, error } = await api.POST("/auth/password", { body });
      if (!response.ok) throw new ApiError(response.status, error);
    },
  });
}
