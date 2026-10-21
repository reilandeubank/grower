import { useId } from "react";
import { X } from "lucide-react";
import { Button } from "../design";
import styles from "./CoverPhotoField.module.css";

/** The add form's staged photo: uploaded after the plant is saved, where it becomes the cover. */
export function CoverPhotoField({ url, onRemove }: { url: string; onRemove: () => void }) {
  const captionId = useId();
  return (
    <figure className={styles.field} aria-labelledby={captionId}>
      <figcaption id={captionId} className={`t-caption ${styles.label}`}>Cover photo</figcaption>
      <img className={styles.photo} src={url} alt="" />
      <Button
        variant="ghost"
        className={styles.remove}
        icon={<X size={20} strokeWidth={1.75} aria-hidden="true" />}
        onClick={onRemove}
      >
        Remove photo
      </Button>
    </figure>
  );
}
