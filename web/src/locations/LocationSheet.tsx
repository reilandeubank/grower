import { useState, type FormEvent } from "react";
import { ApiError, problemMessage, type LightLevel, type Location, type Window } from "../api/client";
import { useCurrentUser } from "../auth/useCurrentUser";
import { BottomSheet, Button, ConfirmButton, ErrorText, SelectField, TextAreaField, TextField } from "../design";
import { CompassPicker } from "../light/CompassPicker";
import { suggestLight } from "../light/rules";
import { LIGHT_LABELS, useDeleteLocation, useSaveLocation } from "./queries";
import styles from "./LocationSheet.module.css";

const LEVEL_OPTIONS = Object.entries(LIGHT_LABELS).map(([value, label]) => ({ value, label }));

export interface LocationSheetProps {
  open: boolean;
  /** Edit this location; without it the sheet adds a new one. */
  location?: Location | undefined;
  onClose: () => void;
  onSaved?: (location: Location) => void;
}

export function LocationSheet({ open, location, onClose, onSaved }: LocationSheetProps) {
  return (
    <BottomSheet open={open} onClose={onClose} title={location ? "Edit location" : "Add location"}>
      <LocationForm
        location={location}
        onSaved={(saved) => {
          onSaved?.(saved);
          onClose();
        }}
        onDeleted={onClose}
      />
    </BottomSheet>
  );
}

// Mounted only while the sheet is open, so each opening starts fresh.
function LocationForm({
  location,
  onSaved,
  onDeleted,
}: {
  location: Location | undefined;
  onSaved: (location: Location) => void;
  onDeleted: () => void;
}) {
  const [name, setName] = useState(location?.name ?? "");
  const [light, setLight] = useState<string>(location?.light ?? "");
  const [windows, setWindows] = useState<Window[]>(location?.windows ?? []);
  const [notes, setNotes] = useState(location?.notes ?? "");
  const [nameError, setNameError] = useState<string>();
  const save = useSaveLocation();
  const remove = useDeleteLocation();
  const { hemisphere } = useCurrentUser().settings;
  // Estimated as you tap, by the same rules the server uses once saved.
  const suggested = suggestLight(windows, hemisphere);

  const onSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!name.trim()) {
      setNameError("Give the location a name.");
      return;
    }
    setNameError(undefined);
    const body = { name: name.trim(), light: (light || null) as LightLevel | null, windows, notes };
    save.mutate({ id: location?.id, body }, { onSuccess: onSaved });
  };

  const conflict = save.error instanceof ApiError && save.error.status === 409;
  const failure = (save.error && !conflict ? save.error : null) ?? remove.error;

  return (
    <form className={styles.form} onSubmit={onSubmit} noValidate>
      <TextField
        label="Name"
        value={name}
        maxLength={60}
        autoComplete="off"
        enterKeyHint="done"
        onChange={(e) => {
          setName(e.target.value);
          save.reset();
        }}
        error={nameError ?? (conflict ? "A location with that name already exists." : undefined)}
      />
      <div className={styles.windows}>
        <CompassPicker value={windows} onChange={setWindows} />
        {/* Always mounted so screen readers hear the estimate change. */}
        <p className={`t-body ${styles.suggested}`} aria-live="polite">
          {suggested && (
            <>
              Suggested light: <strong>{LIGHT_LABELS[suggested]}</strong>
            </>
          )}
        </p>
      </div>
      <SelectField
        label="Light (optional)"
        value={light}
        options={[{ value: "", label: suggested ? "Use the windows' estimate" : "Not set" }, ...LEVEL_OPTIONS]}
        onChange={(e) => setLight(e.target.value)}
      />
      <TextAreaField
        label="Notes"
        rows={3}
        maxLength={2000}
        value={notes}
        onChange={(e) => setNotes(e.target.value)}
      />
      {failure && <ErrorText>{problemMessage(failure)}</ErrorText>}
      <div className={styles.actions}>
        <Button type="submit" fullWidth loading={save.isPending}>
          Save
        </Button>
        {location && (
          <ConfirmButton
            label="Delete location"
            question="Delete this location? Its plants stay, without a location."
            loading={remove.isPending}
            onConfirm={() => remove.mutate(location.id, { onSuccess: onDeleted })}
          />
        )}
      </div>
    </form>
  );
}
