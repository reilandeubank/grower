import createClient from "openapi-fetch";
import type { components, paths } from "./schema";

export type Me = components["schemas"]["Me"];
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
