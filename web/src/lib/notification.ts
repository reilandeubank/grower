// Turns a push payload from the server (internal/push.Message) into what the service
// worker shows. Pure, so it is tested without a service worker.

export interface ShownNotification {
  title: string;
  options: NotificationOptions;
}

const ICON = "/icons/icon-192.png";
// Android draws the badge in the status bar from its alpha channel only.
const BADGE = "/icons/badge-96.png";

/** The notification for a push payload. Anything unreadable still shows something useful. */
export function toNotification(payload: string | undefined): ShownNotification {
  let data: Record<string, unknown> = {};
  try {
    const parsed: unknown = JSON.parse(payload ?? "");
    if (parsed && typeof parsed === "object") data = parsed as Record<string, unknown>;
  } catch {
    // Not JSON: fall back to the defaults below.
  }
  const text = (v: unknown) => (typeof v === "string" && v ? v : undefined);
  return {
    title: text(data.title) ?? "grower",
    options: {
      body: text(data.body) ?? "",
      tag: text(data.tag),
      icon: ICON,
      badge: BADGE,
      data: { url: appPath(data.url) },
    },
  };
}

/** The app path a tap opens: only a path on this site, otherwise Today. */
export function appPath(url: unknown): string {
  return typeof url === "string" && url.startsWith("/") && !url.startsWith("//") ? url : "/";
}
