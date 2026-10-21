import { useEffect, useRef } from "react";
import { Camera, X } from "lucide-react";
import type { GrowthStage } from "../api/client";
import { Button, FileButton, SegmentedControl, TextAreaField, TextField } from "../design";
import { STAGE_OPTIONS } from "./copy";
import type { DraftErrors, MeasurementDraft, PhotoDraft } from "./form";
import styles from "./MeasurementFields.module.css";

export interface MeasurementFieldsProps {
  draft: MeasurementDraft;
  onChange: (draft: MeasurementDraft) => void;
  errors: DraftErrors;
  /** Show the date field, with this as the latest day (a check-in is always today). */
  maxDate?: string | undefined;
}

/** Photo, size, stage and note: shared by the check-in and the Growth tab's measurement sheet. */
export function MeasurementFields({ draft, onChange, errors, maxDate }: MeasurementFieldsProps) {
  const set = (changes: Partial<MeasurementDraft>) => onChange({ ...draft, ...changes });
  return (
    <>
      <PhotoPicker photo={draft.photo} onChange={(photo) => set({ photo })} />
      {maxDate && (
        <TextField
          label="Date"
          type="date"
          max={maxDate}
          value={draft.measuredOn}
          onChange={(e) => set({ measuredOn: e.target.value })}
          error={errors.measuredOn}
        />
      )}
      <fieldset className={styles.size}>
        <legend className="visually-hidden">Size in inches</legend>
        <TextField
          label="Height"
          inputMode="decimal"
          autoComplete="off"
          suffix="in"
          value={draft.height}
          onChange={(e) => set({ height: e.target.value })}
          error={errors.height}
        />
        <TextField
          label="Width"
          inputMode="decimal"
          autoComplete="off"
          suffix="in"
          value={draft.width}
          onChange={(e) => set({ width: e.target.value })}
          error={errors.width}
        />
      </fieldset>
      <SegmentedControl<GrowthStage | "">
        label="Stage"
        options={STAGE_OPTIONS}
        value={draft.stage ?? ""}
        onChange={(stage) => set({ stage: stage || null })}
      />
      <TextAreaField
        label="Note"
        rows={2}
        maxLength={2000}
        placeholder="e.g. Two new leaves"
        value={draft.note}
        onChange={(e) => set({ note: e.target.value })}
      />
    </>
  );
}

function PhotoPicker({ photo, onChange }: { photo: PhotoDraft; onChange: (photo: PhotoDraft) => void }) {
  // The latest preview URL, released when the photo is replaced, removed or the form closes.
  const preview = useRef<string | undefined>(undefined);
  useEffect(() => () => release(preview), []);

  const pick = (file: File) => {
    release(preview);
    // jsdom has no object URLs; the thumbnail is then just empty.
    preview.current = typeof URL.createObjectURL === "function" ? URL.createObjectURL(file) : undefined;
    onChange({ kind: "new", file, preview: preview.current });
  };

  if (photo.kind === "none") {
    return (
      <div className={styles.photo}>
        <FileButton
          icon={<Camera size={20} strokeWidth={1.75} aria-hidden="true" />}
          accept="image/*"
          onFiles={([file]) => file && pick(file)}
        >
          Add photo
        </FileButton>
      </div>
    );
  }
  const src = photo.kind === "saved" ? photo.photo.urls.small : photo.preview;
  return (
    <div className={styles.photo}>
      <div className={styles.thumb}>{src && <img className={styles.image} src={src} alt="Attached photo" />}</div>
      <Button
        variant="ghost"
        icon={<X size={20} strokeWidth={1.75} aria-hidden="true" />}
        onClick={() => {
          release(preview);
          onChange({ kind: "none" });
        }}
      >
        Remove photo
      </Button>
    </div>
  );
}

function release(ref: { current: string | undefined }) {
  if (ref.current) URL.revokeObjectURL(ref.current);
  ref.current = undefined;
}
