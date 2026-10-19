import { useRef, type KeyboardEvent } from "react";
import styles from "./MoistureScale.module.css";

export type MoistureZone = "dry" | "moist" | "wet";

/** The meter's zones: dry 1–3, moist 4–7, wet 8–10. */
export function moistureZone(reading: number): MoistureZone {
  return reading <= 3 ? "dry" : reading <= 7 ? "moist" : "wet";
}

const ZONE_WORDS: Record<MoistureZone, string> = { dry: "Dry", moist: "Moist", wet: "Wet" };

/** "Dry", "Moist" or "Wet". */
export function zoneWord(reading: number): string {
  return ZONE_WORDS[moistureZone(reading)];
}

const READINGS = Array.from({ length: 10 }, (_, i) => i + 1);

// Zone captions over their numbers. A phone wraps the scale into 2 rows of 5, so Moist spans the
// break and gets a caption on each row; the CSS places them and drops the second on one row.
const ZONE_LABELS = [
  { zone: "dry", place: "dry" },
  { zone: "moist", place: "moist" },
  { zone: "moist", place: "moistAgain" },
  { zone: "wet", place: "wet" },
] as const;

export interface MoistureScaleProps {
  /** Names the radio group, e.g. "How wet is the soil?". */
  label: string;
  value: number | null;
  onChange: (reading: number) => void;
  /** Readings above this can't be picked (a threshold stops at 9). */
  max?: number;
  /** Marks the threshold under its number, with a caption. */
  waterAt?: number | undefined;
}

/**
 * The moisture meter's 1–10 scale as a radio group: tap a number, or arrow through them.
 * Colour follows the zone (dry ochre, moist green, wet blue), and each number names its zone.
 */
export function MoistureScale({ label, value, onChange, max = 10, waterAt }: MoistureScaleProps) {
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  // Roving focus: the chosen number is the one tab stop, or the first before anything is chosen.
  const tabStop = value ?? 1;

  const onKeyDown = (e: KeyboardEvent<HTMLButtonElement>, n: number) => {
    const to = { ArrowRight: n + 1, ArrowDown: n + 1, ArrowLeft: n - 1, ArrowUp: n - 1, Home: 1, End: max }[e.key];
    if (to === undefined) return;
    e.preventDefault();
    const next = Math.min(Math.max(to, 1), max);
    onChange(next);
    refs.current[next - 1]?.focus();
  };

  return (
    <div className={styles.scale}>
      <div role="radiogroup" aria-label={label} className={styles.grid}>
        {ZONE_LABELS.map(({ zone, place }) => (
          <span key={place} aria-hidden="true" className={`t-caption ${styles.zone} ${styles[zone]} ${styles[place]}`}>
            {ZONE_WORDS[zone]}
          </span>
        ))}
        {READINGS.map((n) => (
          <button
            key={n}
            ref={(el) => {
              refs.current[n - 1] = el;
            }}
            type="button"
            role="radio"
            aria-checked={n === value}
            aria-label={`${n}, ${zoneWord(n).toLowerCase()}`}
            tabIndex={n === tabStop ? 0 : -1}
            disabled={n > max}
            className={`${styles.reading} ${styles[moistureZone(n)]}`}
            data-threshold={n === waterAt || undefined}
            onClick={() => onChange(n)}
            onKeyDown={(e) => onKeyDown(e, n)}
          >
            <span className={`t-body-strong ${styles.number}`}>{n}</span>
          </button>
        ))}
      </div>
      {waterAt !== undefined && <p className={`t-caption ${styles.caption}`}>Water at {waterAt} or below</p>}
    </div>
  );
}
