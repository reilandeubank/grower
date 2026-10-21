import { useState } from "react";
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Window } from "../api/client";
import { CompassPicker } from "./CompassPicker";

let latest: Window[] = [];
function Harness({ initial = [] }: { initial?: Window[] }) {
  const [value, setValue] = useState(initial);
  return (
    <CompassPicker
      value={value}
      onChange={(windows) => {
        latest = windows;
        setValue(windows);
      }}
    />
  );
}

describe("CompassPicker", () => {
  it("is a named group of eight direction buttons, none pressed at first", () => {
    render(<Harness />);
    expect(screen.getByRole("group", { name: "Windows" })).toBeTruthy();
    const buttons = screen.getAllByRole("button");
    expect(buttons).toHaveLength(8);
    expect(buttons.every((b) => b.getAttribute("aria-pressed") === "false")).toBe(true);
    expect(screen.getByRole("button", { name: "East windows: 0" })).toBeTruthy();
  });

  it("counts up on each tap, shows ×2 and ×3, then clears", async () => {
    render(<Harness />);
    const east = () => screen.getByRole("button", { name: /^East windows/ });
    await userEvent.click(east());
    expect(east().getAttribute("aria-pressed")).toBe("true");
    expect(east().getAttribute("aria-label")).toBe("East windows: 1");
    expect(east().textContent).toBe("E");
    expect(latest).toEqual([{ direction: "e", count: 1 }]);
    await userEvent.click(east());
    expect(east().textContent).toBe("E×2");
    await userEvent.click(east());
    expect(east().getAttribute("aria-label")).toBe("East windows: 3");
    await userEvent.click(east());
    expect(east().getAttribute("aria-pressed")).toBe("false");
    expect(latest).toEqual([]);
  });

  it("keeps other directions and reports windows in compass order", async () => {
    render(<Harness initial={[{ direction: "w", count: 2 }]} />);
    expect(screen.getByRole("button", { name: "West windows: 2" }).getAttribute("aria-pressed")).toBe("true");
    await userEvent.click(screen.getByRole("button", { name: "North windows: 0" }));
    expect(latest).toEqual([
      { direction: "n", count: 1 },
      { direction: "w", count: 2 },
    ]);
  });
});
