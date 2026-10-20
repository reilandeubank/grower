import { useId, useState } from "react";
import { CalendarCheck, Droplet, Gauge, Pencil, Sun } from "lucide-react";
import { problemMessage, type Plant } from "../api/client";
import { Button, ErrorText, useToast } from "../design";
import { applyToast, careGuideLines, hasCareDefaults, sourceLine, type GuideLine } from "./copy";
import { useApplySpecies, useSpecies } from "./queries";
import { SpeciesSheet } from "./SpeciesSheet";
import styles from "./CareGuide.module.css";

const ICONS: Record<GuideLine["kind"], typeof Droplet> = { water: Droplet, light: Sun, waterAt: Gauge };

/** The linked species' care defaults, with Apply and Edit. Nothing while it loads or when unlinked. */
export function CareGuide({ plant }: { plant: Plant }) {
  const species = useSpecies(plant.species_id);
  const apply = useApplySpecies(plant.id);
  const toast = useToast();
  const [editing, setEditing] = useState(false);
  const headingId = useId();

  if (!species.data) return null;
  const s = species.data;
  const lines = careGuideLines(s);
  const onApply = () =>
    apply.mutate(undefined, { onSuccess: ({ before, after }) => toast({ message: applyToast(before, after) }) });

  return (
    <section className={styles.card} aria-labelledby={headingId}>
      <div className={styles.header}>
        <h2 id={headingId} className={`t-caption ${styles.title}`}>
          Care guide
        </h2>
        <Button
          variant="ghost"
          className={styles.edit}
          icon={<Pencil size={20} strokeWidth={1.75} aria-hidden="true" />}
          aria-label="Edit care guide"
          aria-haspopup="dialog"
          onClick={() => setEditing(true)}
        >
          Edit
        </Button>
      </div>
      <p className={styles.names}>
        <span className={`t-body-strong ${styles.scientific}`}>{s.scientific_name}</span>
        {s.common_name && <span className={`t-body ${styles.muted}`}> · {s.common_name}</span>}
      </p>
      {lines.length > 0 ? (
        <ul className={styles.lines}>
          {lines.map(({ kind, text }) => {
            const Icon = ICONS[kind];
            return (
              <li key={kind} className="t-body">
                <Icon className={styles.icon} size={20} strokeWidth={1.75} aria-hidden="true" />
                {text}
              </li>
            );
          })}
        </ul>
      ) : (
        <p className={`t-body ${styles.muted}`}>No care details yet. Edit to add them.</p>
      )}
      <p className={`t-caption ${styles.muted}`}>{sourceLine(s)}</p>
      {hasCareDefaults(s) && (
        <div className={styles.actions}>
          <Button
            variant="secondary"
            fullWidth
            loading={apply.isPending}
            icon={<CalendarCheck size={20} strokeWidth={1.75} aria-hidden="true" />}
            onClick={onApply}
          >
            Apply to care schedule
          </Button>
        </div>
      )}
      {apply.isError && <ErrorText>{problemMessage(apply.error)}</ErrorText>}
      <SpeciesSheet open={editing} species={s} onClose={() => setEditing(false)} />
    </section>
  );
}
