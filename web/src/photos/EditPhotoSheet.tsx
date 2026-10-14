import { useState, type FormEvent } from "react";
import { problemMessage, type Photo } from "../api/client";
import { BottomSheet, Button, ErrorText, TextAreaField, TextField } from "../design";
import { deviceToday } from "../lib/dates";
import { withLocalDate } from "./copy";
import { useUpdatePhoto } from "./queries";
import styles from "./EditPhotoSheet.module.css";

export interface EditPhotoSheetProps {
  open: boolean;
  photo: Photo;
  onClose: () => void;
}

export function EditPhotoSheet({ open, photo, onClose }: EditPhotoSheetProps) {
  return (
    <BottomSheet open={open} onClose={onClose} title="Edit photo">
      <EditPhotoForm photo={photo} onDone={onClose} />
    </BottomSheet>
  );
}

// Mounted only while the sheet is open, so each opening starts fresh.
function EditPhotoForm({ photo, onDone }: { photo: Photo; onDone: () => void }) {
  const takenOn = deviceToday(new Date(photo.taken_at));
  const [caption, setCaption] = useState(photo.caption);
  const [date, setDate] = useState(takenOn);
  const [dateError, setDateError] = useState<string>();
  const save = useUpdatePhoto();

  const onSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!date) {
      setDateError("Pick the day it was taken.");
      return;
    }
    setDateError(undefined);
    // An unchanged day keeps the exact original time.
    const takenAt = date === takenOn ? photo.taken_at : withLocalDate(photo.taken_at, date);
    save.mutate({ id: photo.id, body: { caption: caption.trim(), taken_at: takenAt } }, { onSuccess: onDone });
  };

  return (
    <form className={styles.form} onSubmit={onSubmit} noValidate>
      <TextAreaField label="Caption" value={caption} maxLength={500} rows={3} onChange={(e) => setCaption(e.target.value)} />
      <TextField label="Date" type="date" value={date} onChange={(e) => setDate(e.target.value)} error={dateError} />
      {save.error && <ErrorText>{problemMessage(save.error)}</ErrorText>}
      <Button type="submit" fullWidth loading={save.isPending}>
        Save
      </Button>
    </form>
  );
}
