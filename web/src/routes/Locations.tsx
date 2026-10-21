import { useState } from "react";
import { ArrowLeft, ChevronRight, MapPin, Plus } from "lucide-react";
import type { Location } from "../api/client";
import { AppShell, Button, EmptyState, IconButton, TopBar } from "../design";
import { windowsLine } from "../light/copy";
import { useBack } from "../lib/useBack";
import { LocationSheet } from "../locations/LocationSheet";
import { LIGHT_LABELS, useLocations } from "../locations/queries";
import { LoadError } from "./LoadError";
import { Pending } from "./Pending";
import styles from "./Locations.module.css";

function plantCount(n: number) {
  if (n === 0) return "No plants";
  return n === 1 ? "1 plant" : `${n} plants`;
}

/** "1 N · 1 E · Bright, indirect (est.) · 2 plants": a set light, else the windows' estimate. */
function summary(l: Location) {
  const light = l.light
    ? LIGHT_LABELS[l.light]
    : l.suggested_light && `${LIGHT_LABELS[l.suggested_light]} (est.)`;
  return [windowsLine(l.windows), light, plantCount(l.plant_count)].filter(Boolean).join(" · ");
}

export function Locations() {
  const locations = useLocations();
  const back = useBack("/settings");
  // null: sheet closed; "new": adding; a location: editing it.
  const [editing, setEditing] = useState<Location | "new" | null>(null);

  const addButton = (
    <Button
      variant="secondary"
      icon={<Plus size={20} strokeWidth={1.75} aria-hidden="true" />}
      onClick={() => setEditing("new")}
    >
      Add location
    </Button>
  );

  let content;
  if (locations.isPending) content = <Pending inline />;
  else if (locations.isError) content = <LoadError onRetry={() => void locations.refetch()} />;
  else if (locations.data.length === 0)
    content = (
      <EmptyState
        icon={<MapPin size={28} strokeWidth={1.75} />}
        title="No locations yet"
        body="Add the rooms and windowsills your plants live in, like Kitchen or Bedroom window."
        action={addButton}
      />
    );
  else
    content = (
      <>
        <ul className={styles.list}>
          {locations.data.map((l) => (
            <li key={l.id}>
              <button type="button" className={styles.row} onClick={() => setEditing(l)}>
                <span className={styles.text}>
                  <span className="t-body-strong">{l.name}</span>
                  <span className={`t-caption ${styles.muted}`}>
                    {summary(l)}
                  </span>
                </span>
                <ChevronRight size={20} strokeWidth={1.75} aria-hidden="true" className={styles.muted} />
              </button>
            </li>
          ))}
        </ul>
        {addButton}
      </>
    );

  return (
    <AppShell
      topBar={
        <TopBar
          title="Locations"
          leading={<IconButton aria-label="Back" icon={<ArrowLeft size={20} strokeWidth={1.75} />} onClick={back} />}
        />
      }
    >
      <div className={styles.page}>{content}</div>
      <LocationSheet
        open={editing !== null}
        location={editing === "new" ? undefined : (editing ?? undefined)}
        onClose={() => setEditing(null)}
      />
    </AppShell>
  );
}
