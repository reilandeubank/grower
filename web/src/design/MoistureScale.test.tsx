import { useState } from "react";
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MoistureScale, moistureZone, zoneWord } from "./MoistureScale";

function Harness({ max, waterAt }: { max?: number; waterAt?: number }) {
  const [value, setValue] = useState<number | null>(null);
  return <MoistureScale label="How wet is the soil?" value={value} onChange={setValue} max={max} waterAt={waterAt} />;
}

describe("MoistureScale", () => {
  it("splits the meter into dry 1–3, moist 4–7 and wet 8–10", () => {
    expect([1, 3, 4, 7, 8, 10].map(moistureZone)).toEqual(["dry", "dry", "moist", "moist", "wet", "wet"]);
    expect(zoneWord(6)).toBe("Moist");
  });

  it("is a radio group whose numbers name their zone", async () => {
    render(<Harness waterAt={3} />);
    const group = screen.getByRole("radiogroup", { name: "How wet is the soil?" });
    const radios = screen.getAllByRole("radio");
    expect(group).toBeTruthy();
    expect(radios.map((r) => r.getAttribute("aria-label"))).toEqual([
      "1, dry",
      "2, dry",
      "3, dry",
      "4, moist",
      "5, moist",
      "6, moist",
      "7, moist",
      "8, wet",
      "9, wet",
      "10, wet",
    ]);
    expect(radios.every((r) => r.getAttribute("aria-checked") === "false")).toBe(true);
    expect(screen.getByText("Water at 3 or below")).toBeTruthy();

    await userEvent.click(screen.getByRole("radio", { name: "6, moist" }));
    expect(screen.getByRole("radio", { name: "6, moist" }).getAttribute("aria-checked")).toBe("true");
  });

  it("moves the choice with arrow keys and keeps one tab stop", async () => {
    render(<Harness max={9} />);
    await userEvent.tab();
    expect(document.activeElement).toBe(screen.getByRole("radio", { name: "1, dry" }));
    await userEvent.keyboard("{ArrowRight}{ArrowRight}");
    const three = screen.getByRole("radio", { name: "3, dry" });
    expect(document.activeElement).toBe(three);
    expect(three.getAttribute("aria-checked")).toBe("true");
    await userEvent.keyboard("{ArrowLeft}");
    expect(screen.getByRole("radio", { name: "2, dry" }).getAttribute("aria-checked")).toBe("true");
    // End stops at the cap; 10 can't be picked here.
    await userEvent.keyboard("{End}{ArrowRight}");
    expect(screen.getByRole("radio", { name: "9, wet" }).getAttribute("aria-checked")).toBe("true");
    expect((screen.getByRole("radio", { name: "10, wet" }) as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getAllByRole("radio").filter((r) => r.tabIndex === 0)).toHaveLength(1);
  });
});
