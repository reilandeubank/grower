import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { isOfflineError, OFFLINE_MESSAGE } from "../api/client";
import { useToast } from "../design";

/** One place for every save that fails because the device is offline: a toast, not a crash. */
export function OfflineToast() {
  const queryClient = useQueryClient();
  const toast = useToast();
  useEffect(
    () =>
      queryClient.getMutationCache().subscribe((event) => {
        if (event.type === "updated" && event.action.type === "error" && isOfflineError(event.action.error)) {
          toast({ message: OFFLINE_MESSAGE });
        }
      }),
    [queryClient, toast],
  );
  return null;
}
