import { useState } from "react";
import { describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useCloseOnBack } from "./useCloseOnBack";

function Overlay({ name, children }: { name: string; children?: React.ReactNode }) {
  const [open, setOpen] = useState(false);
  useCloseOnBack(open, () => setOpen(false));
  return (
    <>
      <button onClick={() => setOpen(true)}>Open {name}</button>
      {open && (
        <div role="dialog" aria-label={name}>
          <button onClick={() => setOpen(false)}>Close {name}</button>
          {children}
        </div>
      )}
    </>
  );
}

// jsdom runs history traversal a task later, as browsers do.
const back = () => window.history.back();

describe("useCloseOnBack", () => {
  it("closes on Back and leaves history where it started", async () => {
    render(<Overlay name="sheet" />);
    const start = window.history.length;
    await userEvent.click(screen.getByRole("button", { name: "Open sheet" }));
    expect(window.history.length).toBe(start + 1);

    back();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(window.history.state?.overlay).toBeUndefined();
  });

  it("pops its own entry when closed from the UI", async () => {
    render(<Overlay name="sheet" />);
    await userEvent.click(screen.getByRole("button", { name: "Open sheet" }));
    expect(window.history.state?.overlay).toBe(1);

    await userEvent.click(screen.getByRole("button", { name: "Close sheet" }));
    await waitFor(() => expect(window.history.state?.overlay).toBeUndefined());
    // Reopening pushes a fresh entry at the same depth.
    await userEvent.click(screen.getByRole("button", { name: "Open sheet" }));
    expect(window.history.state?.overlay).toBe(1);
    back();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("closes only the top overlay", async () => {
    render(
      <Overlay name="viewer">
        <Overlay name="sheet" />
      </Overlay>,
    );
    await userEvent.click(screen.getByRole("button", { name: "Open viewer" }));
    await userEvent.click(screen.getByRole("button", { name: "Open sheet" }));
    expect(window.history.state?.overlay).toBe(2);

    back();
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "sheet" })).toBeNull());
    expect(screen.getByRole("dialog", { name: "viewer" })).toBeTruthy();

    back();
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "viewer" })).toBeNull());
  });

  it("keeps the router's state on the entry it pushes", async () => {
    window.history.replaceState({ usr: null, key: "abc", idx: 3 }, "");
    render(<Overlay name="sheet" />);
    await userEvent.click(screen.getByRole("button", { name: "Open sheet" }));
    expect(window.history.state).toEqual({ usr: null, key: "abc", idx: 3, overlay: 1 });
    back();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });
});
