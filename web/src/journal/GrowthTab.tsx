import { useState } from "react";
import { Plus, Ruler } from "lucide-react";
import type { Measurement, Plant } from "../api/client";
import { useToday } from "../care/queries";
import { Button, EmptyState } from "../design";
import { deviceToday, formatDay } from "../lib/dates";
import { PhotoViewer } from "../photos/PhotoViewer";
import { LoadError } from "../routes/LoadError";
import { Pending } from "../routes/Pending";
import { heightPoints, sizeLine } from "./copy";
import { GrowthChart } from "./GrowthChart";
import { MeasurementSheet } from "./MeasurementSheet";
import { usePlantMeasurements } from "./queries";
import styles from "./GrowthTab.module.css";

/** A plant's growth: height over time, then every measurement, newest first. */
export function GrowthTab({ plant }: { plant: Plant }) {
  const measurements = usePlantMeasurements(plant.id);
  const today = useToday().data?.date ?? deviceToday();
  const [sheet, setSheet] = useState<{ measurement?: Measurement } | null>(null);
  const [viewing, setViewing] = useState<string | null>(null);

  if (measurements.isPending) return <Pending inline />;
  if (measurements.isError) return <LoadError onRetry={() => void measurements.refetch()} />;

  const list = measurements.data;
  const points = heightPoints(list);
  const photos = list.flatMap((m) => (m.photo ? [m.photo] : []));
  const add = (
    <Button
      variant="secondary"
      icon={<Plus size={20} strokeWidth={1.75} aria-hidden="true" />}
      onClick={() => setSheet({})}
    >
      Add measurement
    </Button>
  );

  return (
    <div className={styles.tab}>
      {list.length === 0 ? (
        <EmptyState
          as="h2"
          icon={<Ruler size={28} strokeWidth={1.75} />}
          title="No measurements yet"
          body="Note its height, width or stage now and then to see how your plant grows."
          action={add}
        />
      ) : (
        <>
          <div className={styles.add}>{add}</div>
          {points.length >= 2 && <GrowthChart points={points} today={today} />}
          <ul className={styles.list}>
            {list.map((m) => {
              const size = sizeLine(m);
              return (
                <li key={m.id} className={styles.entry}>
                  <button
                    type="button"
                    className={styles.body}
                    aria-haspopup="dialog"
                    onClick={() => setSheet({ measurement: m })}
                  >
                    <span className={`t-caption ${styles.muted}`}>{formatDay(m.measured_on, today)}</span>
                    {size && <span className="t-body-strong">{size}</span>}
                    {m.note && <span className={`t-body ${styles.note}`}>{m.note}</span>}
                    {!size && !m.note && <span className="t-body-strong">Photo</span>}
                  </button>
                  {m.photo && (
                    <button
                      type="button"
                      className={styles.thumb}
                      aria-haspopup="dialog"
                      aria-label={`Photo from ${formatDay(m.measured_on, today)}`}
                      onClick={() => setViewing(m.photo?.id ?? null)}
                    >
                      <img className={styles.image} src={m.photo.urls.small} alt="" loading="lazy" decoding="async" />
                    </button>
                  )}
                </li>
              );
            })}
          </ul>
        </>
      )}
      <MeasurementSheet
        open={sheet !== null}
        plant={plant}
        measurement={sheet?.measurement}
        today={today}
        onClose={() => setSheet(null)}
      />
      <PhotoViewer
        plant={plant}
        photos={photos}
        photoId={viewing}
        onSelect={setViewing}
        onClose={() => setViewing(null)}
      />
    </div>
  );
}
