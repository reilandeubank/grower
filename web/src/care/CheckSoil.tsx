import { useState } from "react";
import { CircleCheck, Droplet } from "lucide-react";
import type { CareTask } from "../api/client";
import { Button, MoistureScale } from "../design";
import { verdict } from "./copy";
import styles from "./CheckSoil.module.css";

export interface CheckSoilProps {
  task: CareTask;
  /** Watered (`done`, with the reading if one was taken) or a reading alone (`checked`). */
  onLog: (kind: "done" | "checked", reading?: number) => void;
  /** Shows "Snooze or skip", for the sheets that offer them. */
  onMore?: (() => void) | undefined;
}

/**
 * A soil check: pick the meter's reading and the verdict shows at once, worked out here from the
 * task's threshold so it needs no round trip. The server then reschedules from the saved reading.
 */
export function CheckSoil({ task, onLog, onMore }: CheckSoilProps) {
  const [reading, setReading] = useState<number | null>(null);
  const result = reading === null ? null : verdict(reading, task.water_at);

  return (
    <>
      <div className={styles.question}>
        <p className="t-body-strong" aria-hidden="true">
          How wet is the soil?
        </p>
        <MoistureScale label="How wet is the soil?" value={reading} onChange={setReading} waterAt={task.water_at} />
      </div>
      <p className={`t-body ${styles.verdict}`} aria-live="polite">
        {result ? (
          <>
            {result.water ? (
              <Droplet className={styles.water} size={20} strokeWidth={1.75} aria-hidden="true" />
            ) : (
              <CircleCheck className={styles.fine} size={20} strokeWidth={1.75} aria-hidden="true" />
            )}
            {result.message}
          </>
        ) : (
          <span className={styles.muted}>Pick the number the meter shows.</span>
        )}
      </p>
      {reading !== null && result?.water ? (
        // Watered sits on the right, under the thumb.
        <div className={styles.pair}>
          <Button variant="secondary" onClick={() => onLog("checked", reading)}>
            Not now
          </Button>
          <Button onClick={() => onLog("done", reading)}>Watered</Button>
        </div>
      ) : (
        <Button fullWidth disabled={reading === null} onClick={() => reading !== null && onLog("checked", reading)}>
          Save reading
        </Button>
      )}
      <div className={styles.links}>
        <Button variant="ghost" onClick={() => onLog("done")}>
          Watered without checking
        </Button>
        {onMore && (
          <Button variant="ghost" onClick={onMore}>
            Snooze or skip
          </Button>
        )}
      </div>
    </>
  );
}
