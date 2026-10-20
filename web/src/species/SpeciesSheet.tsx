import { useState, type FormEvent } from "react";
import { ApiError, problemMessage, type LightLevel, type Species, type SpeciesInput } from "../api/client";
import { BottomSheet, Button, ErrorText, MoistureScale, SelectField, TextAreaField, TextField } from "../design";
import { readingLine } from "../care/copy";
import { LIGHT_LINES } from "./copy";
import { useSaveSpecies } from "./queries";
import styles from "./SpeciesSheet.module.css";

const LIGHT_OPTIONS = [
  { value: "", label: "Not set" },
  ...Object.entries(LIGHT_LINES).map(([value, label]) => ({ value, label })),
];

export interface SpeciesSheetProps {
  open: boolean;
  species: Species;
  onClose: () => void;
}

/** Edits a species' names and care defaults; the server marks it edited by you. */
export function SpeciesSheet({ open, species, onClose }: SpeciesSheetProps) {
  return (
    <BottomSheet open={open} onClose={onClose} title="Edit care guide">
      <SpeciesForm species={species} onDone={onClose} />
    </BottomSheet>
  );
}

/** "" or whole days from 1 to 365; undefined when invalid. */
function parseDays(value: string): number | null | undefined {
  if (!value.trim()) return null;
  const n = Number(value);
  return /^\d+$/.test(value.trim()) && n >= 1 && n <= 365 ? n : undefined;
}

// Mounted only while the sheet is open, so each opening starts fresh.
function SpeciesForm({ species, onDone }: { species: Species; onDone: () => void }) {
  const [scientific, setScientific] = useState(species.scientific_name);
  const [common, setCommon] = useState(species.common_name);
  const [daysText, setDaysText] = useState(species.water_interval_days?.toString() ?? "");
  const [light, setLight] = useState<string>(species.light ?? "");
  const [waterAt, setWaterAt] = useState<number | null>(species.water_at);
  const [notes, setNotes] = useState(species.notes);
  const [errors, setErrors] = useState<{ scientific?: string; days?: string }>({});
  const save = useSaveSpecies();

  const onSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const days = parseDays(daysText);
    const found = {
      scientific: scientific.trim() ? undefined : "Enter the scientific name.",
      days: days === undefined ? "Enter a number of days from 1 to 365, or leave it empty." : undefined,
    };
    setErrors(found);
    if (found.scientific || days === undefined) return;
    const body = {
      scientific_name: scientific.trim(),
      common_name: common.trim(),
      water_interval_days: days,
      light: (light || null) as LightLevel | null,
      water_at: waterAt,
      notes,
    } satisfies SpeciesInput;
    save.mutate({ id: species.id, body }, { onSuccess: onDone });
  };

  const conflict = save.error instanceof ApiError && save.error.status === 409;

  return (
    <form className={styles.form} onSubmit={onSubmit} noValidate>
      <TextField
        label="Scientific name"
        value={scientific}
        maxLength={120}
        autoComplete="off"
        onChange={(e) => {
          setScientific(e.target.value);
          save.reset();
        }}
        error={errors.scientific ?? (conflict ? "A species with that name already exists." : undefined)}
      />
      <TextField
        label="Common name"
        value={common}
        maxLength={120}
        autoComplete="off"
        onChange={(e) => setCommon(e.target.value)}
      />
      <TextField
        label="Water every"
        suffix="days"
        type="number"
        inputMode="numeric"
        min={1}
        max={365}
        value={daysText}
        placeholder="Not set"
        onChange={(e) => setDaysText(e.target.value)}
        error={errors.days}
      />
      <SelectField label="Light" value={light} options={LIGHT_OPTIONS} onChange={(e) => setLight(e.target.value)} />
      <div className={styles.group}>
        <p className={`t-caption ${styles.label}`} aria-hidden="true">
          Water at, for soil checks
          <span className={styles.hint}>{waterAt === null ? "Not set" : readingLine(waterAt)}</span>
        </p>
        <MoistureScale label="Water at, for soil checks" value={waterAt} onChange={setWaterAt} max={9} />
        {waterAt !== null && (
          <Button variant="ghost" className={styles.clear} onClick={() => setWaterAt(null)}>
            Clear
          </Button>
        )}
      </div>
      <TextAreaField
        label="Notes"
        rows={3}
        maxLength={2000}
        value={notes}
        onChange={(e) => setNotes(e.target.value)}
      />
      {save.error && !conflict && <ErrorText>{problemMessage(save.error)}</ErrorText>}
      <Button type="submit" fullWidth loading={save.isPending}>
        Save
      </Button>
    </form>
  );
}
