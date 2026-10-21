import { useEffect, useState } from "react";
import { Check, LoaderCircle, X } from "lucide-react";
import type { IdentifyCandidate } from "../api/client";
import { AppShell, Button, ErrorText, FilterChips, IconButton, TopBar } from "../design";
import { useBack } from "../lib/useBack";
import { confidenceLabel, ORGAN_OPTIONS, useIdentify, type Organ } from "./identify";
import type { PhotoDraft } from "./photoDraft";
import styles from "./IdentifyStep.module.css";

export interface IdentifyStepProps {
  draft: PhotoDraft;
  /** The chosen candidate, or null to go on without a species (also when identification is off). */
  onDone: (candidate: IdentifyCandidate | null) => void;
}

/** "Add from photo", step one: Pl@ntNet's suggestions for the photo. Skipped when not configured. */
export function IdentifyStep({ draft, onDone }: IdentifyStepProps) {
  const [organ, setOrgan] = useState<Organ>("auto");
  const identify = useIdentify(draft, organ);
  const close = useBack("/plants");
  const notConfigured = identify.data?.configured === false;

  useEffect(() => {
    if (notConfigured) onDone(null);
  }, [notConfigured, onDone]);

  const candidates = identify.data?.candidates ?? [];
  let results;
  const failed = identify.isError && !identify.isFetching;
  if (identify.isFetching || notConfigured) {
    results = (
      <p className={`t-body ${styles.status}`}>
        <LoaderCircle className={styles.spin} size={20} strokeWidth={1.75} aria-hidden="true" />
        Identifying…
      </p>
    );
  } else if (failed) {
    results = <ErrorText>Couldn't identify the photo. Try again, or continue without a species.</ErrorText>;
  } else {
    results = (
      <>
        {identify.data?.detail && <p className={`t-body ${styles.muted}`}>{identify.data.detail}</p>}
        {candidates.length > 0 && (
          <ul className={styles.list} aria-label="Suggestions">
            {candidates.map((c) => (
              <li key={c.scientific_name}>
                <CandidateRow candidate={c} onChoose={() => onDone(c)} />
              </li>
            ))}
          </ul>
        )}
      </>
    );
  }

  return (
    <AppShell
      tabBar={false}
      topBar={
        <TopBar
          title="Identify plant"
          leading={<IconButton aria-label="Close" icon={<X size={20} strokeWidth={1.75} />} onClick={close} />}
        />
      }
    >
      <div className={styles.page}>
        <img className={styles.photo} src={draft.url} alt="Your photo" />
        <div className={styles.organ}>
          <p className={`t-caption ${styles.label}`} aria-hidden="true">
            Photo shows
          </p>
          <FilterChips label="Photo shows" options={ORGAN_OPTIONS} value={organ} onChange={(v) => setOrgan(v as Organ)} wrap />
        </div>
        <div className={styles.results} aria-live="polite" aria-busy={identify.isFetching}>
          {results}
        </div>
        <div className={styles.footer}>
          {failed && (
            <Button variant="secondary" fullWidth onClick={() => void identify.refetch()}>
              Try again
            </Button>
          )}
          <Button variant={failed ? "ghost" : "secondary"} fullWidth onClick={() => onDone(null)}>
            {candidates.length > 0 ? "None of these" : "Continue without a species"}
          </Button>
          <p className={`t-caption ${styles.muted}`}>
            The photo is sent to Pl@ntNet to identify it; grower doesn't keep a copy there.
          </p>
        </div>
      </div>
    </AppShell>
  );
}

function CandidateRow({ candidate: c, onChoose }: { candidate: IdentifyCandidate; onChoose: () => void }) {
  const common = c.common_name.trim();
  return (
    <button type="button" className={styles.row} onClick={onChoose}>
      <span className={styles.names}>
        <span className={`t-body-strong ${common ? "" : styles.italic}`}>{common || c.scientific_name}</span>
        {common && <span className={`t-caption ${styles.muted} ${styles.italic}`}>{c.scientific_name}</span>}
      </span>
      <span className={styles.meta}>
        <span className={`t-caption ${styles.muted}`}>{confidenceLabel(c.score)}</span>
        {c.saved_id && (
          <span className={`t-caption ${styles.saved}`}>
            <Check size={14} strokeWidth={2} aria-hidden="true" />
            Saved
          </span>
        )}
      </span>
    </button>
  );
}
