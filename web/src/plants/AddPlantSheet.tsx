import { useState, type ReactNode } from "react";
import { Link, useNavigate } from "react-router";
import { Camera, LoaderCircle, PencilLine } from "lucide-react";
import { BottomSheet, ErrorText } from "../design";
import { uploadErrorMessage } from "../photos/copy";
import { preparePhoto } from "../photos/prepare";
import { stageDraft, type DraftState } from "./photoDraft";
import styles from "./AddPlantSheet.module.css";

/** The Plants FAB's choice: start from a photo (camera or gallery) or fill in the form. */
export function AddPlantSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  const navigate = useNavigate();
  const [preparing, setPreparing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const close = () => {
    setError(null);
    onClose();
  };

  const addFromPhoto = async (file: File) => {
    setPreparing(true);
    setError(null);
    try {
      const draft = stageDraft(await preparePhoto(file));
      // Replace the sheet's history entry, so Back from the form returns to Plants.
      await navigate("/plants/new", { replace: true, state: { draft: draft.id } satisfies DraftState });
    } catch (e) {
      setError(uploadErrorMessage(e instanceof Error ? e : new Error(String(e))));
      setPreparing(false);
    }
  };

  return (
    <BottomSheet open={open} onClose={close} title="Add plant">
      <ul className={styles.choices}>
        <li>
          <label className={styles.choice} aria-busy={preparing || undefined}>
            <Choice
              icon={preparing ? <LoaderCircle className={styles.spin} {...ICON} /> : <Camera {...ICON} />}
              title={preparing ? "Preparing photo…" : "Add from photo"}
              detail="Take or choose a photo"
            />
            <input
              type="file"
              accept="image/*"
              className="visually-hidden"
              disabled={preparing}
              onChange={(e) => {
                const file = e.target.files?.[0];
                // Clear it so picking the same file again still fires a change.
                e.target.value = "";
                if (file) void addFromPhoto(file);
              }}
            />
          </label>
        </li>
        <li>
          <Link to="/plants/new" replace className={styles.choice}>
            <Choice icon={<PencilLine {...ICON} />} title="Enter details" detail="Name, species, location and more" />
          </Link>
        </li>
      </ul>
      {error && <ErrorText>{error}</ErrorText>}
    </BottomSheet>
  );
}

const ICON = { size: 20, strokeWidth: 1.75, "aria-hidden": true } as const;

function Choice({ icon, title, detail }: { icon: ReactNode; title: string; detail: string }) {
  return (
    <>
      <span className={styles.icon}>{icon}</span>
      <span className={styles.text}>
        <span className="t-body-strong">{title}</span>
        <span className={`t-caption ${styles.muted}`}>{detail}</span>
      </span>
    </>
  );
}
