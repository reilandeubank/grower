import { useState, type FormEvent } from "react";
import { problemMessage, type CareTask, type Plant } from "../api/client";
import { useLogEvent } from "../care/queries";
import { Button, ErrorText } from "../design";
import { newDraft, readDraft, type DraftErrors } from "./form";
import { MeasurementFields } from "./MeasurementFields";
import { useSaveEntry } from "./useSaveEntry";
import styles from "./CheckIn.module.css";

export interface CheckInProps {
  task: CareTask;
  plant: Pick<Plant, "id" | "name" | "growth">;
  /** Today's date from the server, for the toast. */
  today: string;
  /** Called once the check-in is saved and the task logged. */
  onDone: () => void;
  /** Shows "Snooze or skip", for the sheets that offer them. */
  onMore?: (() => void) | undefined;
}

/**
 * A check-in: an optional photo, size, stage and note. Saving uploads the photo,
 * saves the measurement linked to it, then logs the task done, in that order; a failure stops
 * there and keeps the sheet open, so the task is only logged once everything else has saved.
 */
export function CheckIn({ task, plant, today, onDone, onMore }: CheckInProps) {
  const prefilledStage = plant.growth?.stage ?? null;
  const [draft, setDraft] = useState(() => newDraft(prefilledStage));
  const [errors, setErrors] = useState<DraftErrors>({});
  const [failure, setFailure] = useState<string>();
  const [busy, setBusy] = useState(false);
  const saveEntry = useSaveEntry(plant.id);
  const log = useLogEvent();

  const onSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    // Nothing filled in is fine: the check-in is then just logged.
    const { body, errors: found } = readDraft(draft, { prefilledStage, allowEmpty: true });
    setErrors(found);
    if (Object.keys(found).length) return;
    setBusy(true);
    setFailure(undefined);
    try {
      if (body) await saveEntry(draft, body);
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
      setBusy(false);
      return;
    }
    try {
      await log.mutateAsync({ task, plantName: plant.name, kind: "done", today });
    } catch (error) {
      setFailure(
        body
          ? "Saved to the growth log, but the check-in isn't marked done yet. Try again."
          : problemMessage(error instanceof Error ? error : new Error(String(error))),
      );
      setBusy(false);
      return;
    }
    onDone();
  };

  return (
    <form className={styles.form} onSubmit={(e) => void onSubmit(e)} noValidate>
      <MeasurementFields draft={draft} onChange={setDraft} errors={errors} />
      {failure && <ErrorText>{failure}</ErrorText>}
      <Button type="submit" fullWidth loading={busy}>
        Save check-in
      </Button>
      {onMore && (
        <div className={styles.links}>
          <Button variant="ghost" onClick={onMore}>
            Snooze or skip
          </Button>
        </div>
      )}
    </form>
  );
}
