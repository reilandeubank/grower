import { ApiError, problemMessage, type Photo } from "../api/client";
import { UnsupportedImageError } from "./prepare";

export interface PhotoMonth {
  /** "2026-10", stable for React keys. */
  key: string;
  /** "October 2026" */
  label: string;
  photos: Photo[];
}

/** Groups photos (already newest first) by the month they were taken, in the device's zone. */
export function groupByMonth(photos: Photo[], timeZone?: string): PhotoMonth[] {
  const keyFormat = new Intl.DateTimeFormat("en-CA", { year: "numeric", month: "2-digit", timeZone });
  const labelFormat = new Intl.DateTimeFormat("en-US", { month: "long", year: "numeric", timeZone });
  const months: PhotoMonth[] = [];
  for (const photo of photos) {
    const taken = new Date(photo.taken_at);
    const key = keyFormat.format(taken);
    const last = months.at(-1);
    if (last?.key === key) last.photos.push(photo);
    else months.push({ key, label: labelFormat.format(taken), photos: [photo] });
  }
  return months;
}

/** "Oct 12, 2026" */
export function formatPhotoDate(takenAt: string, timeZone?: string): string {
  return new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric", year: "numeric", timeZone }).format(
    new Date(takenAt),
  );
}

/** Upload failures: the format message, the server's own words for a rejected file, or the usual copy. */
export function uploadErrorMessage(error: Error): string {
  if (error instanceof UnsupportedImageError) return error.message;
  if (error instanceof ApiError && error.status === 413) {
    return error.problem?.detail ?? "That photo is too large. The limit is 15 MB.";
  }
  return problemMessage(error);
}

/** Moves a timestamp to another calendar day (YYYY-MM-DD) in the device's zone, keeping its time of day. */
export function withLocalDate(takenAt: string, civil: string): string {
  const [year = 1970, month = 1, day = 1] = civil.split("-").map(Number);
  const date = new Date(takenAt);
  date.setFullYear(year, month - 1, day);
  return date.toISOString();
}
