import { CloudOff } from "lucide-react";
import { Button, EmptyState } from "../design";

/** A screen's data failed to load: say so plainly and offer a retry. */
export function LoadError({ onRetry }: { onRetry: () => void }) {
  return (
    <EmptyState
      icon={<CloudOff size={28} strokeWidth={1.75} />}
      title="Can't load this right now"
      body="Check your connection, then try again."
      action={
        <Button variant="secondary" onClick={onRetry}>
          Try again
        </Button>
      }
    />
  );
}
