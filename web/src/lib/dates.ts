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

// Civil dates (YYYY-MM-DD) are calendar days with no time zone. The helpers below do their
// arithmetic at UTC midnight so DST can never shift a day.

function civilToUTC(civil: string): number {
  const [year = 1970, month = 1, day = 1] = civil.split("-").map(Number);
  return Date.UTC(year, month - 1, day);
}

function utcToCivil(ms: number): string {
  return new Date(ms).toISOString().slice(0, 10);
}

/** The device's own calendar date, e.g. for `local_date` on a care event. */
export function deviceToday(now = new Date()): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
}

export function addDays(civil: string, days: number): string {
  return utcToCivil(civilToUTC(civil) + days * 86_400_000);
}

/** Whole days from `from` to `to`; negative when `to` is earlier. */
export function daysBetween(from: string, to: string): number {
  return Math.round((civilToUTC(to) - civilToUTC(from)) / 86_400_000);
}

/** Formats a civil date as-is, whatever the device's zone. */
export function formatCivil(civil: string, options: Intl.DateTimeFormatOptions): string {
  return new Intl.DateTimeFormat("en-US", { ...options, timeZone: "UTC" }).format(new Date(civilToUTC(civil)));
}

/** "Oct 7", or "Oct 7, 2025" when it isn't in the same year as `today`. */
export function formatDay(civil: string, today: string): string {
  const sameYear = civil.slice(0, 4) === today.slice(0, 4);
  return formatCivil(civil, { month: "short", day: "numeric", ...(sameYear ? {} : { year: "numeric" }) });
}

/** "Fri, Oct 17", with the year when it isn't in the same year as `today`. */
export function formatWeekdayDay(civil: string, today: string): string {
  return `${formatCivil(civil, { weekday: "short" })}, ${formatDay(civil, today)}`;
}
