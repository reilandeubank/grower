import { useState } from "react";
import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BottomSheet } from "./BottomSheet";
import { TextField } from "./TextField";

function Harness() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button onClick={() => setOpen(true)}>Add location</button>
      <BottomSheet open={open} onClose={() => setOpen(false)} title="Add location">
        <TextField label="Name" />
      </BottomSheet>
    </>
  );
}

describe("BottomSheet", () => {
  it("focuses the first field on open and returns focus on close", async () => {
    render(<Harness />);
    const opener = screen.getByRole("button", { name: "Add location" });
    await userEvent.click(opener);
    expect(document.activeElement).toBe(screen.getByLabelText("Name"));

    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByLabelText("Name")).toBeNull();
    expect(document.activeElement).toBe(opener);
  });

  it("closes on Esc (the dialog's cancel event) and on a backdrop tap", async () => {
    render(<Harness />);
    await userEvent.click(screen.getByRole("button", { name: "Add location" }));
    const dialog = document.querySelector("dialog")!;
    fireEvent(dialog, new Event("cancel", { cancelable: true }));
    expect(dialog.open).toBe(false);

    await userEvent.click(screen.getByRole("button", { name: "Add location" }));
    expect(dialog.open).toBe(true);
    fireEvent.click(dialog);
    expect(dialog.open).toBe(false);
  });

  it("stays open for clicks inside the sheet", async () => {
    render(<Harness />);
    await userEvent.click(screen.getByRole("button", { name: "Add location" }));
    await userEvent.click(screen.getByLabelText("Name"));
    expect(document.querySelector("dialog")!.open).toBe(true);
  });
});
