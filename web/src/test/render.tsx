import { render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { vi } from "vitest";
import { routes } from "../router";

/** Renders the whole app at `path` with a fresh query cache. */
export function renderApp(path: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { ...utils, router, queryClient };
}

type Handler = (req: Request) => Response | Promise<Response>;

/** Stubs fetch with a "METHOD /path" → handler map; anything else is a 404 so tests fail loudly. */
export function mockApi(handlers: Record<string, Handler>) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const req = input instanceof Request ? input : new Request(input, init);
    const key = `${req.method} ${new URL(req.url).pathname}`;
    const handler = handlers[key];
    return handler ? handler(req) : json(404, { title: "Not found", status: 404 });
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

export function json(status: number, body: unknown): Response {
  const type = status >= 400 ? "application/problem+json" : "application/json";
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": type } });
}

export const me = {
  user: { id: "u1", username: "fern" },
  settings: { timezone: "America/Chicago" },
  version: "v0.1.0-test",
};
