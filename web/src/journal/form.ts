// The measurement form's state and its checks, kept pure for tests.
import type { GrowthStage, Measurement, MeasurementInput, Photo } from "../api/client";

export type PhotoDraft = { kind: "none" } | { kind: "saved"; photo: Photo } | { kind: "new"; file: File; preview: string | undefined };

export interface MeasurementDraft {
  /** YYYY-MM-DD, or "" to let the server use today (check-ins). */
  measuredOn: string;
  height: string;
  width: string;
  stage: GrowthStage | null;
  note: string;
  photo: PhotoDraft;
}

export interface DraftErrors {
  height?: string;
  width?: string;
  measuredOn?: string;
  /** Nothing to save. */
  empty?: string;
}

export const EMPTY_MESSAGE = "Add a size, stage, note or photo.";
const SIZE_MESSAGE = "Enter inches from 0.1 to 600, like 12.5.";

export function newDraft(stage: GrowthStage | null, measuredOn = ""): MeasurementDraft {
  return { measuredOn, height: "", width: "", stage, note: "", photo: { kind: "none" } };
}

export function draftFrom(m: Measurement): MeasurementDraft {
  return {
    measuredOn: m.measured_on,
    height: m.height_in === null ? "" : String(m.height_in),
    width: m.width_in === null ? "" : String(m.width_in),
    stage: m.stage,
    note: m.note,
    photo: m.photo ? { kind: "saved", photo: m.photo } : { kind: "none" },
  };
}

/** Inches from the field: null when blank, undefined when it isn't a size the server takes. */
export function parseInches(text: string): number | null | undefined {
  const t = text.trim().replace(",", ".");
  if (t === "") return null;
  if (!/^\d*\.?\d+$/.test(t)) return undefined;
  const n = Number(t);
  return n >= 0.1 && n <= 600 ? n : undefined;
}

export interface ReadDraftOptions {
  /** The stage the form started with. A check-in doesn't save a stage that was only prefilled. */
  prefilledStage?: GrowthStage | null;
  /** Saving nothing is fine for a check-in (it only logs the task); a measurement needs something. */
  allowEmpty?: boolean;
  /** The latest date allowed, YYYY-MM-DD. */
  today?: string;
}

/**
 * The body to save (without photo_id, which the save fills in after the upload), or null when
 * there's nothing to save, plus any errors to show.
 */
export function readDraft(
  draft: MeasurementDraft,
  { prefilledStage, allowEmpty = false, today }: ReadDraftOptions = {},
): { body: Omit<MeasurementInput, "photo_id"> | null; errors: DraftErrors } {
  const height = parseInches(draft.height);
  const width = parseInches(draft.width);
  const errors: DraftErrors = {};
  if (height === undefined) errors.height = SIZE_MESSAGE;
  if (width === undefined) errors.width = SIZE_MESSAGE;
  if (draft.measuredOn && today && draft.measuredOn > today) errors.measuredOn = "Pick today or an earlier day.";
  if (errors.height || errors.width || errors.measuredOn) return { body: null, errors };

  const note = draft.note.trim();
  const stageChanged = prefilledStage === undefined || draft.stage !== prefilledStage;
  const something = height !== null || width !== null || note !== "" || draft.photo.kind !== "none" || (draft.stage !== null && stageChanged);
  if (!something) return { body: null, errors: allowEmpty ? {} : { empty: EMPTY_MESSAGE } };

  return {
    body: {
      ...(draft.measuredOn ? { measured_on: draft.measuredOn } : {}),
      height_in: height ?? null,
      width_in: width ?? null,
      stage: draft.stage,
      note,
    },
    errors: {},
  };
}
