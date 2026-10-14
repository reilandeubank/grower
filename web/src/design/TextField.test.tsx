import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TextField } from "./TextField";

describe("TextField", () => {
  it("labels the input and describes it with the hint", () => {
    render(<TextField label="Username" hint="The one you were given" />);
    const input = screen.getByLabelText("Username");
    expect(input.getAttribute("aria-invalid")).toBeNull();
    const hint = document.getElementById(input.getAttribute("aria-describedby") ?? "");
    expect(hint?.textContent).toBe("The one you were given");
  });

  it("marks the input invalid and points aria-describedby at the error", () => {
    render(<TextField label="Username" hint="Lowercase" error="Enter your username." />);
    const input = screen.getByLabelText("Username");
    expect(input.getAttribute("aria-invalid")).toBe("true");
    const ids = (input.getAttribute("aria-describedby") ?? "").split(" ");
    const texts = ids.map((id) => document.getElementById(id)?.textContent);
    expect(texts).toEqual(["Lowercase", "Enter your username."]);
  });

  it("toggles password visibility", async () => {
    render(<TextField label="Password" type="password" defaultValue="hunter22" />);
    const input = screen.getByLabelText("Password");
    expect(input.getAttribute("type")).toBe("password");

    await userEvent.click(screen.getByRole("button", { name: "Show password" }));
    expect(input.getAttribute("type")).toBe("text");

    await userEvent.click(screen.getByRole("button", { name: "Hide password" }));
    expect(input.getAttribute("type")).toBe("password");
  });
});
