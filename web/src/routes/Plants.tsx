import { useState } from "react";
import { Link, useSearchParams } from "react-router";
import { Plus, Sprout } from "lucide-react";
import { AppShell, Button, EmptyState, Fab, FilterChips, TopBar } from "../design";
import { useLocations } from "../locations/queries";
import { AddPlantSheet } from "../plants/AddPlantSheet";
import { PlantGrid } from "../plants/PlantGrid";
import { usePlants } from "../plants/queries";
import { LoadError } from "./LoadError";
import { Pending } from "./Pending";
import styles from "./Plants.module.css";

const ALL = "all";
const NONE = "none";

export function Plants() {
  const plants = usePlants();
  const archived = usePlants(true);
  const locations = useLocations();
  const [adding, setAdding] = useState(false);
  // The filter lives in the URL so back and forward step through it.
  const [params, setParams] = useSearchParams();
  const filter = params.get("location") ?? ALL;

  let content;
  if (plants.isPending || locations.isPending) {
    content = <Pending inline />;
  } else if (plants.isError || locations.isError) {
    content = <LoadError onRetry={() => void Promise.all([plants.refetch(), locations.refetch()])} />;
  } else if (plants.data.length === 0) {
    content = (
      <EmptyState
        icon={<Sprout size={28} strokeWidth={1.75} />}
        title="No plants yet"
        body="Add your first plant to start a care schedule."
        action={
          <Button icon={<Plus size={20} strokeWidth={1.75} aria-hidden="true" />} aria-haspopup="dialog" onClick={() => setAdding(true)}>
            Add plant
          </Button>
        }
      />
    );
  } else {
    const options = [
      { value: ALL, label: "All" },
      ...locations.data.map((l) => ({ value: l.id, label: l.name })),
      ...(plants.data.some((p) => !p.location_id) ? [{ value: NONE, label: "No location" }] : []),
    ];
    const shown = plants.data.filter((p) =>
      filter === ALL ? true : filter === NONE ? !p.location_id : p.location_id === filter,
    );
    content = (
      <>
        {locations.data.length > 0 && (
          <FilterChips
            label="Filter by location"
            options={options}
            value={filter}
            onChange={(v) => setParams(v === ALL ? {} : { location: v })}
          />
        )}
        {shown.length > 0 ? (
          <PlantGrid plants={shown} locations={locations.data} />
        ) : (
          <p className={`t-body ${styles.muted}`}>No plants here yet.</p>
        )}
      </>
    );
  }

  const archivedCount = archived.data?.length ?? 0;
  return (
    <AppShell topBar={<TopBar title="Plants" />}>
      <div className={styles.page}>
        {content}
        {archivedCount > 0 && (
          <Link to="/plants/archived" className={`t-body ${styles.archivedLink}`}>
            Archived ({archivedCount})
          </Link>
        )}
      </div>
      {/* The empty state has its own Add plant button. */}
      {plants.data?.length !== 0 && <Fab label="Add plant" onClick={() => setAdding(true)} />}
      <AddPlantSheet open={adding} onClose={() => setAdding(false)} />
    </AppShell>
  );
}
