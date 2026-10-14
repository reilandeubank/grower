import { useState } from "react";
import { ImagePlus, Images, LoaderCircle } from "lucide-react";
import type { Plant } from "../api/client";
import { EmptyState, FileButton, useToast } from "../design";
import { LoadError } from "../routes/LoadError";
import { Pending } from "../routes/Pending";
import { formatPhotoDate, groupByMonth, uploadErrorMessage } from "./copy";
import { PhotoViewer } from "./PhotoViewer";
import { usePlantPhotos, useUploadPhoto } from "./queries";
import styles from "./PhotosTab.module.css";

/** A plant's photo timeline by month, with upload and a full-screen viewer. */
export function PhotosTab({ plant }: { plant: Plant }) {
  const photos = usePlantPhotos(plant.id);
  const upload = useUploadPhoto(plant.id);
  const toast = useToast();
  const [uploading, setUploading] = useState(0);
  const [viewing, setViewing] = useState<string | null>(null);

  // One at a time keeps memory down: each photo is decoded at full size before it's scaled.
  const addPhotos = async (files: File[]) => {
    setUploading((n) => n + files.length);
    let added = 0;
    let failure: Error | undefined;
    for (const file of files) {
      try {
        await upload.mutateAsync(file);
        added++;
      } catch (error) {
        failure = error instanceof Error ? error : new Error(String(error));
      }
      setUploading((n) => n - 1);
    }
    if (failure) toast({ message: uploadErrorMessage(failure) });
    else toast({ message: added === 1 ? "Photo added." : `${added} photos added.` });
  };

  if (photos.isPending) return <Pending inline />;
  if (photos.isError) return <LoadError onRetry={() => void photos.refetch()} />;

  const add = (
    <FileButton icon={<ImagePlus size={20} strokeWidth={1.75} aria-hidden="true" />} accept="image/*" multiple onFiles={addPhotos}>
      Add photo
    </FileButton>
  );

  if (photos.data.length === 0 && uploading === 0) {
    return (
      <EmptyState
        as="h2"
        icon={<Images size={28} strokeWidth={1.75} />}
        title="No photos yet"
        body="Add a photo now and then to see how your plant grows."
        action={add}
      />
    );
  }

  return (
    <div className={styles.tab}>
      <div className={styles.add}>{add}</div>
      {uploading > 0 && (
        <ul className={styles.grid}>
          {Array.from({ length: uploading }, (_, i) => (
            <li key={i} className={styles.tile}>
              <LoaderCircle className={styles.spinner} size={24} strokeWidth={1.75} aria-hidden="true" />
              <span className="visually-hidden">Uploading photo</span>
            </li>
          ))}
        </ul>
      )}
      {groupByMonth(photos.data).map((month) => (
        <section key={month.key} className={styles.month}>
          <h2 className="t-heading">{month.label}</h2>
          <ul className={styles.grid}>
            {month.photos.map((photo) => (
              <li key={photo.id}>
                <button
                  type="button"
                  className={styles.tile}
                  aria-haspopup="dialog"
                  aria-label={`Photo from ${formatPhotoDate(photo.taken_at)}`}
                  onClick={() => setViewing(photo.id)}
                >
                  <img className={styles.image} src={photo.urls.small} alt="" loading="lazy" decoding="async" />
                </button>
              </li>
            ))}
          </ul>
        </section>
      ))}
      <PhotoViewer
        plant={plant}
        photos={photos.data}
        photoId={viewing}
        onSelect={setViewing}
        onClose={() => setViewing(null)}
      />
    </div>
  );
}
