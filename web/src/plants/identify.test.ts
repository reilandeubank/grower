import { describe, expect, it } from "vitest";
import { candidatePrefill, confidenceLabel } from "./identify";

describe("confidenceLabel", () => {
  it.each([
    [0.82, "Likely · 82%"],
    [0.6, "Likely · 60%"],
    [0.596, "Likely · 60%"],
    [0.59, "Possible · 59%"],
    [0.41, "Possible · 41%"],
    [0.25, "Possible · 25%"],
    [0.24, "Unlikely · 24%"],
    [0.09, "Unlikely · 9%"],
    [0, "Unlikely · 0%"],
    [1.2, "Likely · 100%"],
  ])("%s reads %s", (score, label) => {
    expect(confidenceLabel(score)).toBe(label);
  });
});

describe("candidatePrefill", () => {
  it("links a saved species and capitalises the common name", () => {
    expect(
      candidatePrefill({ scientific_name: "Monstera deliciosa", common_name: "swiss cheese plant", score: 0.9, saved_id: "s1" }),
    ).toEqual({
      values: { name: "Swiss cheese plant", species: "Monstera deliciosa", speciesId: "s1" },
      newSpecies: null,
    });
  });

  it("asks for a manual species when it isn't saved, and leaves the name empty without a common name", () => {
    expect(candidatePrefill({ scientific_name: "Ficus lyrata", common_name: "", score: 0.5, saved_id: null })).toEqual({
      values: { name: "", species: "Ficus lyrata", speciesId: "" },
      newSpecies: { scientific_name: "Ficus lyrata", common_name: "", notes: "" },
    });
  });
});
