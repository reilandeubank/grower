/** Formats like "Tuesday, Oct 14" in the given IANA zone (falls back to the device zone). */
export function formatTodayHeading(now: Date, timeZone?: string): string {
  return format(now, { weekday: "long", month: "short", day: "numeric" }, timeZone);
}

/** "Good morning" / "Good afternoon" / "Good evening" for the hour in the given zone. */
export function greeting(now: Date, timeZone?: string): string {
  const hour = Number(format(now, { hour: "numeric", hourCycle: "h23" }, timeZone));
  if (hour >= 5 && hour < 12) return "Good morning";
  if (hour >= 12 && hour < 18) return "Good afternoon";
  return "Good evening";
}

function format(now: Date, options: Intl.DateTimeFormatOptions, timeZone?: string): string {
  try {
    return new Intl.DateTimeFormat("en-US", { ...options, timeZone }).format(now);
  } catch {
    // An unknown zone name throws a RangeError; the device zone is a fine fallback.
    return new Intl.DateTimeFormat("en-US", options).format(now);
  }
}

/** Formats a civil date (YYYY-MM-DD) as "Mar 2024". */
export function formatMonthYear(civil: string): string {
  const [year, month] = civil.split("-").map(Number);
  return new Intl.DateTimeFormat("en-US", { month: "short", year: "numeric", timeZone: "UTC" }).format(
    new Date(Date.UTC(year ?? 1970, (month ?? 1) - 1, 1)),
  );
}
