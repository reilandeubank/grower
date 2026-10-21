import createClient from "openapi-fetch";
import type { components, paths } from "./schema";

export type Me = components["schemas"]["Me"];
export type Settings = components["schemas"]["Settings"];
export type StorageUse = components["schemas"]["Storage"];
export type PotType = components["schemas"]["PotType"];
export type Problem = components["schemas"]["Problem"];

/** Typed client for the grower API. The session cookie rides along automatically. */
export const api = createClient<paths>({
  baseUrl: "/api/v1",
  credentials: "same-origin",
  headers: { "Content-Type": "application/json" },
  // Resolve fetch at call time so tests can stub globalThis.fetch.
  fetch: (request) => globalThis.fetch(request),
});

/** An API failure with its HTTP status; screens map statuses to copy. */
export class ApiError extends Error {
  readonly status: number;
  readonly problem: Problem | undefined;

  constructor(status: number, problem?: Problem) {
    super(problem?.title ?? `Request failed with status ${status}`);
    this.name = "ApiError";
    this.status = status;
    this.problem = problem;
  }
}

export type Location = components["schemas"]["Location"];
export type LocationInput = components["schemas"]["LocationInput"];
export type LightLevel = components["schemas"]["LightLevel"];
export type Direction = components["schemas"]["Direction"];
export type Window = components["schemas"]["Window"];
export type WindowDistance = components["schemas"]["WindowDistance"];
export type EffectiveLight = components["schemas"]["EffectiveLight"];
export type Plant = components["schemas"]["Plant"];
export type PlantInput = components["schemas"]["PlantInput"];
export type PlantUpdate = components["schemas"]["PlantUpdate"];
export type TaskKind = components["schemas"]["TaskKind"];
export type Anchor = components["schemas"]["Anchor"];
export type CareTask = components["schemas"]["CareTask"];
export type CareTaskInput = components["schemas"]["CareTaskInput"];
export type EventKind = components["schemas"]["EventKind"];
export type HistoryEntry = components["schemas"]["HistoryEntry"];
export type LoggedEvent = components["schemas"]["LoggedEvent"];
export type Today = components["schemas"]["Today"];
export type TodayItem = components["schemas"]["TodayItem"];
export type Photo = components["schemas"]["Photo"];
export type PhotoInput = components["schemas"]["PhotoInput"];
export type PhotoUpload = components["schemas"]["PhotoUpload"];
export type Species = components["schemas"]["Species"];
export type SpeciesInput = components["schemas"]["SpeciesInput"];
export type SpeciesSource = components["schemas"]["SpeciesSource"];
export type SpeciesCandidate = components["schemas"]["SpeciesCandidate"];
export type SpeciesLookup = components["schemas"]["SpeciesLookup"];
export type GrowthStage = components["schemas"]["GrowthStage"];
export type Measurement = components["schemas"]["Measurement"];
export type MeasurementInput = components["schemas"]["MeasurementInput"];
export type IdentifyCandidate = components["schemas"]["IdentifyCandidate"];
export type IdentifyRequest = components["schemas"]["IdentifyRequest"];

/** Unwraps an openapi-fetch result, throwing an ApiError for anything but a body-bearing success. */
export function unwrap<T>({ data, error, response }: { data?: T; error?: Problem; response: Response }): T {
  if (error !== undefined || data === undefined) throw new ApiError(response.status, error);
  return data;
}

export const OFFLINE_MESSAGE = "You're offline. Try again when you're connected.";

/** A request that never reached the server because the device is offline. */
export function isOfflineError(error: unknown): boolean {
  return !(error instanceof ApiError) && !navigator.onLine;
}

/** Copy for a failed save: the server's own detail for validation errors, plain words otherwise. */
export function problemMessage(error: Error): string {
  if (error instanceof ApiError) {
    if ((error.status === 400 || error.status === 409) && error.problem?.detail) return error.problem.detail;
    if (error.status === 404) return "This was deleted somewhere else.";
    return "Something went wrong on the server. Try again in a moment.";
  }
  if (isOfflineError(error)) return OFFLINE_MESSAGE;
  return "Can't reach grower. Check your connection, then try again.";
}
