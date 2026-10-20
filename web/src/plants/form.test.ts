import { describe, expect, it } from "vitest";
import type { Plant } from "../api/client";
import { potLine } from "./copy";
import { plantInput, plantValues, validatePlant } from "./form";

const plant: Plant = {
  id: "p1",
  name: "Monstera",
  species: "",
  species_id: null,
  location_id: null,
  acquired_on: null,
  notes: "",
  archived: false,
  cover: null,
  created_at: "2026-10-14T12:00:00Z",
  pot_type: "self_watering",
  pot_size_in: 8,
  drainage: false,
  soil: "Chunky mix",
};

describe("plant form mapping", () => {
  it("round-trips pot and soil through the form's strings", () => {
    const values = plantValues(plant);
    expect(values).toMatchObject({ potType: "self_watering", potSize: "8", drainage: "no", soil: "Chunky mix" });
    expect(plantInput(values)).toMatchObject({
      pot_type: "self_watering",
      pot_size_in: 8,
      drainage: false,
      soil: "Chunky mix",
    });
  });

  it("sends null for pot fields left unset", () => {
    const input = plantInput({ ...plantValues(), name: "Fern", soil: "  " });
    expect(input).toMatchObject({ pot_type: null, pot_size_in: null, drainage: null, soil: "" });
  });

  it("allows pot sizes from 0.5 to 48 inches", () => {
    const check = (potSize: string) => validatePlant({ ...plantValues(), name: "Fern", potSize }).potSize;
    expect(check("")).toBeUndefined();
    expect(check("0.5")).toBeUndefined();
    expect(check("48")).toBeUndefined();
    expect(check("0.4")).toBe("Use a size from 0.5 to 48 inches.");
    expect(check("49")).toBe("Use a size from 0.5 to 48 inches.");
  });
});

describe("potLine", () => {
  const none = { pot_type: null, pot_size_in: null, drainage: null, soil: "" };

  it("is null when nothing is recorded", () => {
    expect(potLine(none)).toBeNull();
  });

  it("joins what is recorded, in sentence case", () => {
    expect(potLine({ pot_type: "terracotta", pot_size_in: 6, drainage: true, soil: "aroid mix" })).toBe(
      "6 in terracotta pot · drains · aroid mix",
    );
    expect(potLine({ ...none, pot_type: "self_watering" })).toBe("Self-watering pot");
    expect(potLine({ ...none, pot_type: "other", pot_size_in: 4.5 })).toBe("4.5 in pot");
    expect(potLine({ ...none, drainage: false })).toBe("No drainage");
    expect(potLine({ ...none, soil: "Cactus mix" })).toBe("Cactus mix");
  });
});
