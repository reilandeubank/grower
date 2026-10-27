// Turns a push payload from the server (internal/push.Message) into what the service
// worker shows, and handles the reminder's "Remind me at" action. Pure apart from the
// injected fetch, so it is tested without a service worker.
import { formatClock } from "./dates";

/** A button on a notification. TypeScript's DOM types don't list `actions` yet. */
export interface NotificationAction {
  action: string;
  title: string;
}

export interface ShownNotification {
  title: string;
  options: NotificationOptions & { actions?: NotificationAction[] };
}

/** The action that sends today's reminder again at the user's later time. */
export const LATER_ACTION = "later";

const ICON = "/icons/icon-192.png";
// Android draws the badge in the status bar from its alpha channel only.
const BADGE = "/icons/badge-96.png";

/** The notification for a push payload. Anything unreadable still shows something useful. */
export function toNotification(payload: string | undefined): ShownNotification {
  let data: Record<string, unknown> = {};
  try {
    const parsed: unknown = JSON.parse(payload ?? "");
    if (parsed && typeof parsed === "object")
      data = parsed as Record<string, unknown>;
  } catch {
    // Not JSON: fall back to the defaults below.
  }
  const text = (v: unknown) => (typeof v === "string" && v ? v : undefined);
  const date = text(data.date);
  const later = text(data.later_time);
  const canPostpone =
    !!date &&
    /^\d{4}-\d{2}-\d{2}$/.test(date) &&
    !!later &&
    /^\d{2}:\d{2}$/.test(later);
  return {
    title: text(data.title) ?? "grower",
    options: {
      body: text(data.body) ?? "",
      tag: text(data.tag),
      icon: ICON,
      badge: BADGE,
      data: canPostpone
        ? { url: appPath(data.url), date }
        : { url: appPath(data.url) },
      ...(canPostpone && {
        actions: [
          { action: LATER_ACTION, title: `Remind me at ${formatClock(later)}` },
        ],
      }),
    },
  };
}

/** The app path a tap opens: only a path on this site, otherwise Today. */
export function appPath(url: unknown): string {
  return typeof url === "string" && url.startsWith("/") && !url.startsWith("//")
    ? url
    : "/";
}

/**
 * Asks the server to send the reminder for `date` again at the later time. Returns the
 * notification to show when that fails, or null when it worked.
 */
export async function remindLater(
  date: string,
  fetchFn: typeof fetch = fetch,
): Promise<ShownNotification | null> {
  let body: string;
  try {
    const res = await fetchFn("/api/v1/push/remind-later", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ date }),
    });
    if (res.ok) return null;
    if (res.status === 401) {
      body = "Open grower and sign in, then try again.";
    } else {
      const problem: unknown = await res.json().catch(() => null);
      const detail =
        problem && typeof problem === "object"
          ? (problem as { detail?: unknown }).detail
          : undefined;
      body =
        typeof detail === "string" && detail
          ? detail
          : "Something went wrong. Open grower to see what's due.";
    }
  } catch {
    body =
      "Couldn't reach grower. Check your connection, then open grower to see what's due.";
  }
  return {
    title: "Couldn't move the reminder",
    options: {
      body,
      tag: "digest",
      icon: ICON,
      badge: BADGE,
      data: { url: "/" },
    },
  };
}
