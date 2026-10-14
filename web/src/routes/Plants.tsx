import { Sprout } from "lucide-react";
import { AppShell, EmptyState, TopBar } from "../design";

export function Plants() {
  return (
    <AppShell topBar={<TopBar title="Plants" />}>
      <EmptyState
        icon={<Sprout size={28} strokeWidth={1.75} />}
        title="No plants yet"
        body="Adding plants is coming in the next update."
      />
    </AppShell>
  );
}
