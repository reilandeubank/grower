import { Archive, ArrowLeft } from "lucide-react";
import { AppShell, EmptyState, IconButton, TopBar } from "../design";
import { useLocations } from "../locations/queries";
import { useBack } from "../lib/useBack";
import { PlantGrid } from "../plants/PlantGrid";
import { usePlants } from "../plants/queries";
import { LoadError } from "./LoadError";
import { Pending } from "./Pending";

export function ArchivedPlants() {
  const plants = usePlants(true);
  const locations = useLocations();
  const back = useBack("/plants");

  let content;
  if (plants.isPending) content = <Pending inline />;
  else if (plants.isError) content = <LoadError onRetry={() => void plants.refetch()} />;
  else if (plants.data.length === 0)
    content = (
      <EmptyState
        icon={<Archive size={28} strokeWidth={1.75} />}
        title="No archived plants"
        body="Plants you archive show up here, and you can restore them anytime."
      />
    );
  else content = <PlantGrid plants={plants.data} locations={locations.data} />;

  return (
    <AppShell
      topBar={
        <TopBar
          title="Archived plants"
          leading={<IconButton aria-label="Back" icon={<ArrowLeft size={20} strokeWidth={1.75} />} onClick={back} />}
        />
      }
    >
      {content}
    </AppShell>
  );
}
