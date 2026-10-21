import { useQuery } from "@tanstack/react-query";
import { api, unwrap, type IdentifyCandidate, type IdentifyRequest, type SpeciesInput } from "../api/client";
import type { PlantValues } from "./form";
import type { PhotoDraft } from "./photoDraft";

export type Organ = IdentifyRequest["organ"];

export const ORGAN_OPTIONS: { value: Organ; label: string }[] = [
  { value: "auto", label: "Not sure" },
  { value: "leaf", label: "Leaf" },
  { value: "flower", label: "Flower" },
  { value: "fruit", label: "Fruit" },
  { value: "bark", label: "Bark" },
];

/** "Likely · 82%", "Possible · 41%", "Unlikely · 9%". The word follows the rounded percent shown. */
export function confidenceLabel(score: number): string {
  const percent = Math.round(Math.min(1, Math.max(0, score)) * 100);
  const word = percent >= 60 ? "Likely" : percent >= 25 ? "Possible" : "Unlikely";
  return `${word} · ${percent}%`;
}

/**
 * Pl@ntNet suggestions for the draft's photo. A query keyed by draft and organ, so switching back
 * to an organ already tried reuses its answer instead of spending another identification.
 */
export function useIdentify(draft: PhotoDraft, organ: Organ) {
  return useQuery({
    queryKey: ["identify", draft.id, organ],
    queryFn: async () => {
      const form = new FormData();
      form.append("file", draft.photo.blob, "photo.jpg");
      form.append("organ", organ);
      return unwrap(
        await api.POST("/identify", {
          // The schema types `file` as a string; the real body is multipart FormData.
          body: form as unknown as IdentifyRequest,
          // Drop the client's JSON default so the browser sets multipart with its boundary.
          headers: { "Content-Type": null },
        }),
      );
    },
    staleTime: Infinity,
    retry: false,
    refetchOnWindowFocus: false,
  });
}

/** What a chosen candidate fills in on the add form. */
export interface CandidatePrefill {
  values: Pick<PlantValues, "name" | "species" | "speciesId">;
  /** A manual species to create on save, when the candidate isn't saved yet. */
  newSpecies: SpeciesInput | null;
}

export function candidatePrefill(c: IdentifyCandidate): CandidatePrefill {
  const common = c.common_name.trim();
  return {
    values: {
      name: common.charAt(0).toUpperCase() + common.slice(1),
      species: c.scientific_name,
      speciesId: c.saved_id ?? "",
    },
    newSpecies: c.saved_id
      ? null
      : { scientific_name: c.scientific_name, common_name: c.common_name, notes: "" },
  };
}
