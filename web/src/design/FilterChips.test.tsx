import { useState } from "react";
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FilterChips } from "./FilterChips";

function Harness() {
  const [value, setValue] = useState("all");
  return (
    <FilterChips
      label="Filter by location"
      options={[
        { value: "all", label: "All" },
        { value: "k", label: "Kitchen" },
      ]}
      value={value}
      onChange={setValue}
    />
  );
}

describe("FilterChips", () => {
  it("marks exactly one chip pressed and moves the selection on tap", async () => {
    render(<Harness />);
    const all = screen.getByRole("button", { name: "All" });
    const kitchen = screen.getByRole("button", { name: "Kitchen" });
    expect(screen.getByRole("group", { name: "Filter by location" })).toBeTruthy();
    expect(all.getAttribute("aria-pressed")).toBe("true");
    expect(kitchen.getAttribute("aria-pressed")).toBe("false");

    await userEvent.click(kitchen);
    expect(all.getAttribute("aria-pressed")).toBe("false");
    expect(kitchen.getAttribute("aria-pressed")).toBe("true");
  });
});
