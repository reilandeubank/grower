import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { Check, ChevronLeft, ChevronRight, ImageIcon, Pencil, X } from "lucide-react";
import { problemMessage, type Photo, type Plant } from "../api/client";
import { useCloseOnBack } from "../lib/useCloseOnBack";
import { Button, ConfirmButton, ErrorText, IconButton, useToast } from "../design";
import { formatPhotoDate } from "./copy";
import { EditPhotoSheet } from "./EditPhotoSheet";
import { useDeletePhoto, useSetCover } from "./queries";
import styles from "./PhotoViewer.module.css";

export interface PhotoViewerProps {
  plant: Plant;
  /** Newest first, as the timeline shows them. */
  photos: Photo[];
  /** The photo on show; the viewer is closed when it's null or not in `photos`. */
  photoId: string | null;
  onSelect: (photoId: string | null) => void;
  onClose: () => void;
}

/** A full-screen photo on a native modal <dialog>: Esc, Android back and the close button leave it. */
export function PhotoViewer({ plant, photos, photoId, onSelect, onClose }: PhotoViewerProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const index = photos.findIndex((p) => p.id === photoId);
  const photo = photos[index];
  const open = photo !== undefined;
  useCloseOnBack(open, onClose);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!open || !dialog) return;
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    dialog.showModal();
    return () => {
      dialog.close();
      opener?.focus();
    };
  }, [open]);

  return (
    <dialog
      ref={dialogRef}
      className={styles.viewer}
      aria-label="Photo"
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
    >
      {photo && (
        <ViewerBody
          key={photo.id}
          plant={plant}
          photo={photo}
          position={`${index + 1} of ${photos.length}`}
          newer={photos[index - 1]?.id}
          older={photos[index + 1]?.id}
          onSelect={onSelect}
          onClose={onClose}
        />
      )}
    </dialog>
  );
}

interface ViewerBodyProps {
  plant: Plant;
  photo: Photo;
  position: string;
  newer: string | undefined;
  older: string | undefined;
  onSelect: (photoId: string | null) => void;
  onClose: () => void;
}

// Keyed by photo, so the delete confirmation and errors reset when the photo changes.
function ViewerBody({ plant, photo, position, newer, older, onSelect, onClose }: ViewerBodyProps) {
  const [editing, setEditing] = useState(false);
  const setCover = useSetCover(plant.id);
  const remove = useDeletePhoto();
  const toast = useToast();
  const isCover = plant.cover?.id === photo.id;
  const date = formatPhotoDate(photo.taken_at);
  const failure = setCover.error ?? remove.error;

  const onKeyDown = (e: KeyboardEvent) => {
    if (editing) return;
    const next = e.key === "ArrowLeft" ? newer : e.key === "ArrowRight" ? older : undefined;
    if (next) onSelect(next);
  };

  const onDelete = () =>
    remove.mutate(photo.id, {
      onSuccess: () => {
        toast({ message: "Photo deleted." });
        onSelect(older ?? newer ?? null);
      },
    });

  return (
    <div className={styles.body} onKeyDown={onKeyDown}>
      <div className={styles.header}>
        <IconButton aria-label="Close" icon={<X size={20} strokeWidth={1.75} />} onClick={onClose} />
        <span className={`t-caption ${styles.muted}`}>{position}</span>
      </div>

      <div className={styles.stage}>
        <img className={styles.image} src={photo.urls.medium} alt={photo.caption || `${plant.name}, ${date}`} decoding="async" />
        {newer && (
          <IconButton
            className={`${styles.nav} ${styles.prev}`}
            aria-label="Newer photo"
            icon={<ChevronLeft size={20} strokeWidth={1.75} />}
            onClick={() => onSelect(newer)}
          />
        )}
        {older && (
          <IconButton
            className={`${styles.nav} ${styles.next}`}
            aria-label="Older photo"
            icon={<ChevronRight size={20} strokeWidth={1.75} />}
            onClick={() => onSelect(older)}
          />
        )}
      </div>

      <div className={styles.info}>
        <div className={styles.text}>
          <p className={`t-caption ${styles.muted}`}>{date}</p>
          {photo.caption && <p className={`t-body ${styles.caption}`}>{photo.caption}</p>}
        </div>
        <div className={styles.actions}>
          <Button
            variant="secondary"
            icon={
              isCover ? (
                <Check size={20} strokeWidth={1.75} aria-hidden="true" />
              ) : (
                <ImageIcon size={20} strokeWidth={1.75} aria-hidden="true" />
              )
            }
            disabled={isCover}
            loading={setCover.isPending}
            onClick={() => setCover.mutate(photo.id)}
          >
            {isCover ? "Cover photo" : "Set as cover"}
          </Button>
          <Button
            variant="secondary"
            icon={<Pencil size={20} strokeWidth={1.75} aria-hidden="true" />}
            aria-haspopup="dialog"
            onClick={() => setEditing(true)}
          >
            Edit caption
          </Button>
        </div>
        {failure && <ErrorText>{problemMessage(failure)}</ErrorText>}
        <ConfirmButton
          label="Delete photo"
          question="Delete this photo? This can't be undone."
          loading={remove.isPending}
          onConfirm={onDelete}
        />
      </div>

      <EditPhotoSheet open={editing} photo={photo} onClose={() => setEditing(false)} />
    </div>
  );
}
