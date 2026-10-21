import { formatDay } from "../lib/dates";
import { chartGeometry, type ChartBox } from "./chart";
import { chartSummary, inches, type HeightPoint } from "./copy";
import styles from "./GrowthChart.module.css";

// The plot's own units; it scales to the column's width and keeps its shape.
const BOX: ChartBox = { width: 320, height: 112, pad: 8 };

/**
 * Height over time: one brand line on two labelled gridlines (the lowest and highest heights).
 * Screen readers get the summary; the timeline below lists every value.
 */
export function GrowthChart({ points, today }: { points: HeightPoint[]; today: string }) {
  const g = chartGeometry(points, BOX);
  const first = points[0];
  const last = points.at(-1);
  const line = g.points.map((p) => `${p.x},${p.y}`).join(" ");
  // Labels sit in HTML beside the plot, at the gridlines' heights, so text never stretches.
  const at = (y: number) => ({ top: `${(y / BOX.height) * 100}%` });

  return (
    <figure className={styles.figure}>
      <figcaption className={`t-caption ${styles.title}`}>Height</figcaption>
      <div className={styles.plot}>
        <svg
          className={styles.svg}
          viewBox={`0 0 ${BOX.width} ${BOX.height}`}
          role="img"
          aria-label={chartSummary(points, today)}
        >
          <line className={styles.grid} x1={0} x2={BOX.width} y1={g.yMax} y2={g.yMax} />
          {g.max !== g.min && <line className={styles.grid} x1={0} x2={BOX.width} y1={g.yMin} y2={g.yMin} />}
          <polyline className={styles.line} points={line} />
          {g.points.map((p, i) => (
            <circle key={i} className={styles.dot} cx={p.x} cy={p.y} r={4} />
          ))}
        </svg>
        <div className={`t-caption ${styles.yAxis}`} aria-hidden="true">
          <span style={at(g.yMax)}>{inches(g.max)}</span>
          {g.max !== g.min && <span style={at(g.yMin)}>{inches(g.min)}</span>}
        </div>
      </div>
      {first && last && (
        <div className={`t-caption ${styles.xAxis}`} aria-hidden="true">
          <span>{formatDay(first.on, today)}</span>
          {last.on !== first.on && <span>{formatDay(last.on, today)}</span>}
        </div>
      )}
    </figure>
  );
}
