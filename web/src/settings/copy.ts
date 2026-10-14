import type { StorageUse } from "../api/client";
import { daysBetween, deviceToday, formatDay } from "../lib/dates";

const count = (n: number, one: string) => `${n} ${one}${n === 1 ? "" : "s"}`;

/** "820 KB", "1.2 MB", "63 MB", "2.4 GB": decimal units, as phones show storage. */
export function formatBytes(bytes: number): string {
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / 1000;
  let unit = 0;
  while (value >= 1000 && unit < units.length - 1) {
    value /= 1000;
    unit++;
  }
  const shown = value < 10 ? Math.round(value * 10) / 10 : Math.round(value);
  return `${shown} ${units[unit]}`;
}

/** "12 plants · 48 photos (63 MB) · database 1.2 MB". Plants are left out until they've loaded. */
export function storageSummary(storage: StorageUse, plants: number | undefined): string {
  const photos =
    storage.photo_count > 0
      ? `${count(storage.photo_count, "photo")} (${formatBytes(storage.photo_bytes)})`
      : "No photos";
  return [plants === undefined ? null : count(plants, "plant"), photos, `database ${formatBytes(storage.database_bytes)}`]
    .filter(Boolean)
    .join(" · ");
}

/** "Today at 3:04 AM", "Yesterday at 3:04 AM", "Oct 12 at 3:04 AM", or "No backup yet". */
export function lastBackupLine(at: string | null, now = new Date()): string {
  if (!at) return "No backup yet";
  const when = new Date(at);
  const today = deviceToday(now);
  const day = deviceToday(when);
  const time = new Intl.DateTimeFormat("en-US", { hour: "numeric", minute: "2-digit" }).format(when);
  const ago = daysBetween(day, today);
  const date = ago === 0 ? "Today" : ago === 1 ? "Yesterday" : formatDay(day, today);
  return `${date} at ${time}`;
}
