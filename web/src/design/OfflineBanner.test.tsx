import { describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider, useMutation } from "@tanstack/react-query";
import userEvent from "@testing-library/user-event";
import { OfflineToast } from "../lib/OfflineToast";
import { OfflineBanner } from "./OfflineBanner";
import { ToastProvider } from "./Toast";

function goOffline(offline: boolean) {
  vi.spyOn(navigator, "onLine", "get").mockReturnValue(!offline);
  act(() => {
    window.dispatchEvent(new Event(offline ? "offline" : "online"));
  });
}

const BANNER = "You're offline. Showing what was loaded last.";

describe("OfflineBanner", () => {
  it("appears while offline and goes when the connection is back", () => {
    render(<OfflineBanner />);
    expect(screen.queryByText(BANNER)).toBeNull();
    goOffline(true);
    expect(screen.getByRole("status").textContent).toBe(BANNER);
    goOffline(false);
    expect(screen.queryByText(BANNER)).toBeNull();
  });
});

function Save() {
  const save = useMutation({
    mutationFn: () => fetch("/api/v1/plants", { method: "POST" }),
    networkMode: "always",
  });
  return <button onClick={() => save.mutate()}>Save</button>;
}

describe("OfflineToast", () => {
  it("explains a save that failed because the device is offline", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ToastProvider>
          <OfflineToast />
          <Save />
        </ToastProvider>
      </QueryClientProvider>,
    );
    goOffline(true);
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("You're offline. Try again when you're connected.")).toBeTruthy();
  });
});
