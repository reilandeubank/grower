import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import {
  api,
  ApiError,
  isOfflineError,
  unwrap,
  type CareTask,
  type CareTaskInput,
  type EventKind,
  type LoggedEvent,
  type Today,
} from "../api/client";
import { plantHistoryKey, plantTasksKey, todayKey } from "../api/keys";
import { useToast } from "../design";
import { deviceToday } from "../lib/dates";
import { doneToast, skipToast, snoozeToast } from "./copy";

export function useToday() {
  return useQuery({
    queryKey: todayKey,
    queryFn: async () => unwrap(await api.GET("/today")),
  });
}

export function usePlantTasks(plantId: string) {
  return useQuery({
    queryKey: plantTasksKey(plantId),
    queryFn: async () =>
      unwrap(await api.GET("/plants/{id}/tasks", { params: { path: { id: plantId } } })).items,
  });
}

export function usePlantHistory(plantId: string) {
  return useQuery({
    queryKey: plantHistoryKey(plantId),
    queryFn: async () =>
      unwrap(await api.GET("/plants/{id}/events", { params: { path: { id: plantId } } })).items,
  });
}

/** Everything a care event can change: Today, and the plant's tasks and history. */
function refreshCare(queryClient: QueryClient, plantId: string) {
  return Promise.all(
    [todayKey, plantTasksKey(plantId), plantHistoryKey(plantId)].map((queryKey) =>
      queryClient.invalidateQueries({ queryKey }),
    ),
  );
}

/** Creates when `id` is absent, otherwise replaces the schedule. */
export function useSaveTask(plantId: string) {
  const queryClient = useQueryClient();
  return useMutation<CareTask, Error, { id?: string | undefined; body: CareTaskInput }>({
    mutationFn: async ({ id, body }) =>
      unwrap(
        id
          ? await api.PUT("/tasks/{id}", { params: { path: { id } }, body })
          : await api.POST("/plants/{id}/tasks", { params: { path: { id: plantId } }, body }),
      ),
    onSuccess: () => refreshCare(queryClient, plantId),
  });
}

export function useDeleteTask(plantId: string) {
  const queryClient = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: async (id) => {
      const { response, error } = await api.DELETE("/tasks/{id}", { params: { path: { id } } });
      if (!response.ok) throw new ApiError(response.status, error);
    },
    onSuccess: () => refreshCare(queryClient, plantId),
  });
}

export interface LogEventVars {
  task: CareTask;
  plantName: string;
  kind: EventKind;
  /** Required for `snoozed`: a date after today. */
  snoozeUntil?: string;
  /** Today's date from the server, for the toast's wording. */
  today: string;
}

function removeFromToday(today: Today | undefined, taskId: string): Today | undefined {
  if (!today) return today;
  const keep = (items: Today["due"]) => items.filter((i) => i.task.id !== taskId);
  return { ...today, overdue: keep(today.overdue), due: keep(today.due), upcoming: keep(today.upcoming) };
}

/**
 * Records done, skipped or snoozed. The task leaves Today at once and comes back if the save
 * fails; on success a toast offers Undo. All the work lives in the hook's own callbacks, so a
 * completion still lands if the screen has moved on by the time it's sent.
 */
export function useLogEvent() {
  const queryClient = useQueryClient();
  const toast = useToast();

  // A plain function rather than a mutation: the toast can outlive the screen that showed it.
  const undo = async (eventId: string, plantId: string) => {
    try {
      const { response } = await api.DELETE("/events/{id}", { params: { path: { id: eventId } } });
      // 404: already undone or deleted, which is what Undo wanted anyway.
      if (!response.ok && response.status !== 404) throw new Error(String(response.status));
    } catch {
      toast({ message: "Couldn't undo that. Try again in a moment." });
    }
    await refreshCare(queryClient, plantId);
  };

  return useMutation<LoggedEvent, Error, LogEventVars, { previous: Today | undefined }>({
    mutationFn: async ({ task, kind, snoozeUntil }) =>
      unwrap(
        await api.POST("/tasks/{id}/events", {
          params: { path: { id: task.id } },
          // The device's date, so a tap just before midnight counts for that day.
          body: { kind, note: "", local_date: deviceToday(), ...(snoozeUntil ? { snooze_until: snoozeUntil } : {}) },
        }),
      ),
    onMutate: async ({ task }) => {
      await queryClient.cancelQueries({ queryKey: todayKey });
      const previous = queryClient.getQueryData<Today>(todayKey);
      queryClient.setQueryData<Today>(todayKey, (t) => removeFromToday(t, task.id));
      return { previous };
    },
    onError: (error, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(todayKey, context.previous);
      // Offline failures get the app-wide offline toast (OfflineToast).
      if (!isOfflineError(error)) toast({ message: "Couldn't save that. Check your connection, then try again." });
    },
    onSuccess: ({ event }, { task, plantName, kind, snoozeUntil, today }) => {
      const message =
        kind === "done"
          ? doneToast(task.kind, task.label, plantName)
          : kind === "skipped"
            ? skipToast(task.kind, task.label, plantName)
            : snoozeToast(snoozeUntil ?? event.snooze_until ?? today, today);
      toast({ message, action: { label: "Undo", onClick: () => void undo(event.id, task.plant_id) } });
    },
    onSettled: (_data, _error, { task }) => refreshCare(queryClient, task.plant_id),
  });
}
