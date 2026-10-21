import type { PreparedPhoto } from "../photos/prepare";

/** A photo picked for "Add from photo", waiting for its plant to be saved. */
export interface PhotoDraft {
  id: string;
  photo: PreparedPhoto;
  /** An object URL for previews; revoked when the draft is replaced or cleared. */
  url: string;
}

/** The router state that hands a draft to the add form. */
export interface DraftState {
  draft: string;
}

// Blobs don't belong in the URL, so the form gets only the draft's id (in router state) and looks
// it up here. One draft at a time; after a reload it's gone and the form simply opens empty.
let current: PhotoDraft | null = null;

export function stageDraft(photo: PreparedPhoto): PhotoDraft {
  clearDraft();
  current = { id: Date.now().toString(36), photo, url: URL.createObjectURL(photo.blob) };
  return current;
}

/** The draft named by router state, if it's still staged. */
export function getDraft(state: unknown): PhotoDraft | null {
  const id = (state as Partial<DraftState> | null)?.draft;
  return current && current.id === id ? current : null;
}

export function clearDraft() {
  if (current) URL.revokeObjectURL(current.url);
  current = null;
}
