import { useId } from "react";
import { Compass } from "lucide-react";
import type { Direction, Window } from "../api/client";
import { DIRECTION_NAMES, DIRECTIONS } from "./copy";
import styles from "./CompassPicker.module.css";

/** Taps go 0 → 1 → … → MAX_TAPS → 0. More windows than this facing one way is rare. */
export const MAX_TAPS = 3;

// Reading order of the 3×3 grid; null is the centre.
const GRID: (Direction | null)[] = ["nw", "n", "ne", "w", null, "e", "sw", "s", "se"];

export interface CompassPickerProps {
  value: Window[];
  onChange: (windows: Window[]) => void;
}

/** Windows by the way they face: tap a direction to add one, again for more, once more to clear. */
export function CompassPicker({ value, onChange }: CompassPickerProps) {
  const hintId = useId();
  const count = (d: Direction) => value.find((w) => w.direction === d)?.count ?? 0;

  const tap = (d: Direction) => {
    const next = count(d) >= MAX_TAPS ? 0 : count(d) + 1;
    onChange(
      DIRECTIONS.map((direction) => ({ direction, count: direction === d ? next : count(direction) })).filter(
        (w) => w.count > 0,
      ),
    );
  };

  return (
    <fieldset className={styles.fieldset} aria-describedby={hintId}>
      <legend className={`t-caption ${styles.legend}`}>Windows</legend>
      <div className={styles.grid}>
        {GRID.map((d) =>
          d === null ? (
            <span key="centre" className={styles.centre} aria-hidden="true">
              <Compass size={20} strokeWidth={1.75} />
            </span>
          ) : (
            <button
              key={d}
              type="button"
              className={`t-caption ${styles.direction}`}
              aria-pressed={count(d) > 0}
              aria-label={`${DIRECTION_NAMES[d]} windows: ${count(d)}`}
              onClick={() => tap(d)}
            >
              {d.toUpperCase()}
              {count(d) > 1 && <span className={styles.count}>×{count(d)}</span>}
            </button>
          ),
        )}
      </div>
      <p id={hintId} className={`t-caption ${styles.hint}`}>
        Tap the way each window faces. Tap again for 2 or 3, once more to clear.
      </p>
    </fieldset>
  );
}
