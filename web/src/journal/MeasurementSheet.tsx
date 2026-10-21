import { useState, type FormEvent } from "react";
import { problemMessage, type Measurement, type Plant } from "../api/client";
import { BottomSheet, Button, ConfirmButton, ErrorText, useToast } from "../design";
import { draftFrom, newDraft, readDraft, type DraftErrors } from "./form";
import { MeasurementFields } from "./MeasurementFields";
import { useDeleteMeasurement } from "./queries";
import { useSaveEntry } from "./useSaveEntry";
import styles from "./CheckIn.module.css";

export interface MeasurementSheetProps {
  open: boolean;
  plant: Pick<Plant, "id" | "growth">;
  /** Edit this one; without it the sheet adds a new measurement. */
  measurement?: Measurement | undefined;
  /** Today's date: the default and the latest allowed. */
  today: string;
  onClose: () => void;
}

/** Add, edit or delete a measurement: the check-in's fields plus a date, without logging a task. */
export function MeasurementSheet({ open, onClose, ...rest }: MeasurementSheetProps) {
  return (
    <BottomSheet open={open} onClose={onClose} title={rest.measurement ? "Edit measurement" : "Add measurement"}>
      <MeasurementForm {...rest} onDone={onClose} />
    </BottomSheet>
  );
}

// Mounted only while the sheet is open, so each opening starts fresh.
function MeasurementForm({
  plant,
  measurement,
  today,
  onDone,
}: Omit<MeasurementSheetProps, "open" | "onClose"> & { onDone: () => void }) {
  const [draft, setDraft] = useState(() =>
    measurement ? draftFrom(measurement) : newDraft(plant.growth?.stage ?? null, today),
  );
  const [errors, setErrors] = useState<DraftErrors>({});
  const [failure, setFailure] = useState<string>();
  const [busy, setBusy] = useState(false);
  const saveEntry = useSaveEntry(plant.id);
  const remove = useDeleteMeasurement(plant.id);
  const toast = useToast();

  const onSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const { body, errors: found } = readDraft(draft, { today });
    setErrors(found);
    if (!body) return;
    setBusy(true);
    setFailure(undefined);
    try {
      await saveEntry(draft, body, measurement?.id);
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
      setBusy(false);
      return;
    }
    toast({ message: measurement ? "Measurement saved." : "Measurement added." });
    onDone();
  };

  const onDelete = (id: string) =>
    remove.mutate(id, {
      onSuccess: () => {
        toast({ message: "Measurement deleted." });
        onDone();
      },
    });

  const shown = failure ?? (remove.error ? problemMessage(remove.error) : errors.empty);
  return (
    <form className={styles.form} onSubmit={(e) => void onSubmit(e)} noValidate>
      <MeasurementFields draft={draft} onChange={setDraft} errors={errors} maxDate={today} />
      {shown && <ErrorText>{shown}</ErrorText>}
      <Button type="submit" fullWidth loading={busy}>
        Save
      </Button>
      {measurement && (
        <ConfirmButton
          label="Delete measurement"
          question={measurement.photo ? "Delete this measurement? Its photo stays in Photos." : "Delete this measurement?"}
          loading={remove.isPending}
          onConfirm={() => onDelete(measurement.id)}
        />
      )}
    </form>
  );
}
