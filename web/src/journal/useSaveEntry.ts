import { useRef } from "react";
import { problemMessage, type Measurement, type MeasurementInput } from "../api/client";
import { uploadErrorMessage } from "../photos/copy";
import { useUploadPhoto } from "../photos/queries";
import type { MeasurementDraft } from "./form";
import { useSaveMeasurement } from "./queries";

/**
 * Uploads a newly picked photo, then saves the measurement linked to it. Failures throw an Error
 * whose message is ready to show. A retry after a partial failure reuses what already saved: the
 * same file isn't uploaded twice, and a measurement created once is updated rather than duplicated.
 */
export function useSaveEntry(plantId: string) {
  const upload = useUploadPhoto(plantId);
  const save = useSaveMeasurement(plantId);
  const uploaded = useRef<{ file: File; id: string } | null>(null);
  const created = useRef<string | undefined>(undefined);

  return async (draft: MeasurementDraft, body: Omit<MeasurementInput, "photo_id">, id?: string): Promise<Measurement> => {
    let photoId: string | null = draft.photo.kind === "saved" ? draft.photo.photo.id : null;
    if (draft.photo.kind === "new") {
      const { file } = draft.photo;
      if (uploaded.current?.file !== file) {
        try {
          uploaded.current = { file, id: (await upload.mutateAsync(file)).id };
        } catch (error) {
          throw new Error(`The photo didn't upload, so nothing was saved. ${uploadErrorMessage(asError(error))}`, {
            cause: error,
          });
        }
      }
      photoId = uploaded.current.id;
    }
    try {
      const saved = await save.mutateAsync({ id: id ?? created.current, body: { ...body, photo_id: photoId } });
      if (!id) created.current = saved.id;
      return saved;
    } catch (error) {
      throw new Error(problemMessage(asError(error)), { cause: error });
    }
  };
}

const asError = (e: unknown) => (e instanceof Error ? e : new Error(String(e)));
