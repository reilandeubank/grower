import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, unwrap, type Me, type Settings } from "../api/client";
import { plantsKey, todayKey } from "../api/keys";
import { meKey } from "../auth/hooks";

export const storageKey = ["storage"] as const;

export function useStorage() {
  return useQuery({
    queryKey: storageKey,
    queryFn: async () => unwrap(await api.GET("/storage")),
  });
}

/**
 * Saves a change to the settings, keeping the rest as they are. Saves run one at a time, in
 * order. A new time zone moves "today" and a new hemisphere moves seasonal due dates, so Today
 * and the plants (with their task lists) refresh too.
 */
export function useUpdateSettings() {
  const queryClient = useQueryClient();
  return useMutation<Settings, Error, Partial<Settings>>({
    scope: { id: "settings" },
    mutationFn: async (change) => {
      const current = queryClient.getQueryData<Me | null>(meKey)?.settings;
      if (!current) throw new Error("Settings are not loaded");
      return unwrap(await api.PUT("/settings", { body: { ...current, ...change } }));
    },
    onSuccess: (settings) => {
      queryClient.setQueryData<Me | null>(meKey, (me) => (me ? { ...me, settings } : me));
      return Promise.all([todayKey, plantsKey].map((queryKey) => queryClient.invalidateQueries({ queryKey })));
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
