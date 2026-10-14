// The daily reminder on this device: browser push subscription plus the server's record of it.
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, unwrap } from "../api/client";

/** What the Notifications section shows. */
export type PushState = "unsupported" | "denied" | "off" | "on";

export const pushStateKey = ["push-state"] as const;

function supported(): boolean {
  return "serviceWorker" in navigator && "PushManager" in window && "Notification" in window;
}

/** This device's subscription, if any. No service worker yet means none. */
async function currentSubscription(): Promise<PushSubscription | null> {
  const registration = await navigator.serviceWorker.getRegistration();
  return (await registration?.pushManager.getSubscription()) ?? null;
}

export async function readPushState(): Promise<PushState> {
  if (!supported()) return "unsupported";
  if (Notification.permission === "denied") return "denied";
  return (await currentSubscription()) ? "on" : "off";
}

/** The server's VAPID key, URL-safe base64, as the bytes PushManager.subscribe takes. */
function keyBytes(base64url: string): Uint8Array<ArrayBuffer> {
  const base64 = base64url
    .replace(/-/g, "+")
    .replace(/_/g, "/")
    .padEnd(Math.ceil(base64url.length / 4) * 4, "=");
  return Uint8Array.from(atob(base64), (c) => c.charCodeAt(0));
}

async function enable(): Promise<void> {
  // Declining (or dismissing) the prompt is not an error: the state shows the outcome.
  if ((await Notification.requestPermission()) !== "granted") return;
  // No worker means no push (as under `make dev`); fail rather than wait for one forever.
  const registration = await navigator.serviceWorker.getRegistration();
  if (!registration) throw new Error("No service worker");
  const { public_key } = unwrap(await api.GET("/push/vapid-key"));
  const subscription = await registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: keyBytes(public_key),
  });
  const { endpoint = "", keys = {} } = subscription.toJSON();
  const { response, error } = await api.POST("/push/subscriptions", {
    body: { endpoint, keys: { p256dh: keys.p256dh ?? "", auth: keys.auth ?? "" } },
  });
  if (!response.ok) {
    // Keep the device and the server in step: no reminder half turned on.
    await subscription.unsubscribe();
    throw new ApiError(response.status, error);
  }
}

export async function disable(): Promise<void> {
  const subscription = await currentSubscription();
  if (!subscription) return;
  const { response, error } = await api.DELETE("/push/subscriptions", { body: { endpoint: subscription.endpoint } });
  if (!response.ok) throw new ApiError(response.status, error);
  await subscription.unsubscribe();
}

export function usePushState() {
  return useQuery({ queryKey: pushStateKey, queryFn: readPushState, networkMode: "always" });
}

/** Turns the daily reminder on (asking for permission first) or off for this device. */
export function useSetReminder() {
  const queryClient = useQueryClient();
  return useMutation<void, Error, boolean>({
    mutationFn: (on) => (on ? enable() : disable()),
    onSettled: () => queryClient.invalidateQueries({ queryKey: pushStateKey }),
  });
}

export function useSendTestPush() {
  return useMutation<void, Error, void>({
    mutationFn: async () => {
      const { response, error } = await api.POST("/push/test");
      if (!response.ok) throw new ApiError(response.status, error);
    },
  });
}
