import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, unwrap, type ApiToken, type ApiTokenCreated } from "../api/client";
import { daysBetween, deviceToday, formatDay } from "../lib/dates";

export const apiTokensKey = ["api-tokens"] as const;

/** The path Home Assistant (or anything holding a token) reads. */
export const HOME_ASSISTANT_PATH = "/api/v1/integrations/home-assistant";

export function useApiTokens() {
  return useQuery({
    queryKey: apiTokensKey,
    queryFn: async () => unwrap(await api.GET("/api-tokens")).items,
  });
}

/** Creates a read-only token. The secret is only in this response, so the caller shows it once. */
export function useCreateApiToken() {
  const queryClient = useQueryClient();
  return useMutation<ApiTokenCreated, Error, string>({
    mutationFn: async (label) => unwrap(await api.POST("/api-tokens", { body: { label } })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: apiTokensKey }),
  });
}

/** Revokes a token. Already gone (404) counts as done. */
export function useRevokeApiToken() {
  const queryClient = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: async (id) => {
      const { response, error } = await api.DELETE("/api-tokens/{id}", { params: { path: { id } } });
      if (!response.ok && response.status !== 404) throw new ApiError(response.status, error);
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: apiTokensKey }),
  });
}

/** "Today", "Yesterday" or "Oct 12", on the device's calendar. */
function dayWord(at: string, now: Date): string {
  const today = deviceToday(now);
  const day = deviceToday(new Date(at));
  const ago = daysBetween(day, today);
  return ago === 0 ? "today" : ago === 1 ? "yesterday" : formatDay(day, today);
}

/** "Created Oct 12 · Last used today", or "Created today · Never used". */
export function tokenLine(token: Pick<ApiToken, "created_at" | "last_used_at">, now = new Date()): string {
  const created = dayWord(token.created_at, now);
  const used = token.last_used_at ? `Last used ${dayWord(token.last_used_at, now)}` : "Never used";
  return `Created ${created} · ${used}`;
}
