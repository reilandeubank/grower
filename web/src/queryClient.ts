import { QueryClient } from "@tanstack/react-query";

export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      // Offline, a fetch can still be answered by the service worker's cache, so try it once
      // rather than pausing. Saves never queue: they fail at once and OfflineToast says why.
      queries: { retry: 1, refetchOnWindowFocus: true, networkMode: "offlineFirst" },
      mutations: { retry: false, networkMode: "always" },
    },
  });
}
